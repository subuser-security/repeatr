package git

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/inconshreveable/log15"
	"github.com/polydawn/gosh"
	"github.com/spacemonkeygo/errors"
	"github.com/spacemonkeygo/errors/try"

	"polydawn.net/repeatr/io"
)

const Kind = integrity.TransmatKind("git")

var _ integrity.Transmat = &GitTransmat{}

type GitTransmat struct {
	workPath string
}

var _ integrity.TransmatFactory = New

func New(workPath string) integrity.Transmat {
	err := os.MkdirAll(workPath, 0755)
	if err != nil {
		panic(integrity.TransmatError.New("Unable to set up workspace: %s", err))
	}
	workPath, err = filepath.Abs(workPath)
	if err != nil {
		panic(integrity.TransmatError.New("Unable to set up workspace: %s", err))
	}
	return &GitTransmat{workPath}
}

var git gosh.Command = gosh.Gosh(
	"git",
	gosh.NullIO,
	gosh.Opts{Env: map[string]string{
		"GIT_CONFIG_NOSYSTEM": "true",
		"HOME":                "/dev/null",
	}},
)

/*
	Git transmats plonk down the contents of one commit (or tree) as a filesystem.

	A fileset materialized by git does *not* include the `.git` dir by default,
	since those files are not themselves part of what's described by the hash.

	Git effectively "filters" out several attributes -- permissions are only loosely
	respected (execution only), file timestamps are undefined, uid/gid bits
	are not tracked, xattrs are not tracked, etc.  If you desired defined values,
	*you must still configure materialization to use a filter* (particularly for
	file timestamps, since they will otherwise be allowed to vary from one
	materialization to the next(!)).

	Git also allows for several other potential pitfalls with lossless data
	transmission: git cannot transmit empty directories.  This can be a major pain.
	Typical workarounds include creating a ".gitkeep" file in the empty directory.
	Gitignore files may also inadventantly cause trouble.  Transmat.Materialize
	will act *consistently*, but it does not overcome these issues in git
	(doing so would require additional metadata or protocol extensions).

	This transmat is *not* currently well optimized, and should generally be assumed
	to be re-cloning on all materializations -- specifically, it is not smart
	enough to recognize requests for different commits and trees from the
	same repos in order to save reclones.
*/
func (t *GitTransmat) Materialize(
	kind integrity.TransmatKind,
	dataHash integrity.CommitID,
	siloURIs []integrity.SiloURI,
	log log15.Logger,
	options ...integrity.MaterializerConfigurer,
) integrity.Arena {
	var arena gitArena
	try.Do(func() {
		// Basic validation and config
		//config := integrity.EvaluateConfig(options...)
		if kind != Kind {
			panic(errors.ProgrammerError.New("This transmat supports definitions of type %q, not %q", Kind, kind))
		}

		// Emit git version.
		// Until we get a reasonably static version linked&contained, this is going to be an ongoing source of potential trouble.
		gitv := git.Bake("version").CombinedOutput()
		log.Info("using `git version`:", "v", strings.TrimSpace(gitv))

		// Ping silos
		if len(siloURIs) < 1 {
			panic(integrity.ConfigError.New("Materialization requires at least one data source!"))
			// Note that it's possible a caching layer will satisfy things even without data sources...
			//  but if that was going to happen, it already would have by now.
		}
		// Our policy is to take the first path that exists.
		//  This lets you specify a series of potential locations,
		//  and if one is unavailable we'll just take the next.
		var warehouse *Warehouse
		for _, uri := range siloURIs {
			wh := NewWarehouse(uri)
			pong := wh.Ping()
			if pong == nil {
				log.Info("git transmat: connected to remote warehouse", "remote", uri)
				warehouse = wh
				break
			} else {
				log.Info("Warehouse unavailable, skipping",
					"remote", uri,
					"reason", pong.Message(),
				)
			}
		}
		if warehouse == nil {
			panic(integrity.WarehouseUnavailableError.New("No warehouses were available!"))
		}

		// Create staging arena to produce data into.
		var err error
		arena.gitDirPath, err = ioutil.TempDir(t.workPath, "")
		if err != nil {
			panic(integrity.TransmatError.New("Unable to create arena: %s", err))
		}
		arena.workDirPath, err = ioutil.TempDir(t.workPath, "")
		if err != nil {
			panic(integrity.TransmatError.New("Unable to create arena: %s", err))
		}

		// From now on, all our git commands will have these overriden paths:
		// This gives us a working tree without ".git".
		git := git.Bake(
			gosh.Opts{Env: map[string]string{
				"GIT_DIR":       arena.gitDirPath,
				"GIT_WORK_TREE": arena.workDirPath,
			}},
		)

		// Clone!
		// TODO make sure all the check hard modes are enabled
		git.Bake(
			"clone", "--bare", "--", warehouse.url, arena.gitDirPath,
		).RunAndReport()
		log.Info("git transmat: clone complete")

		// Checkout the interesting commit.
		buf := &bytes.Buffer{}
		p := git.Bake(
			"checkout", string(dataHash), // FIXME dear god, whitelist this to make sure it looks like a hash.
			gosh.Opts{Cwd: arena.workDirPath},
			gosh.Opts{OkExit: gosh.AnyExit},
			gosh.Opts{Err: buf, Out: buf},
		).Run()
		if bytes.HasPrefix(buf.Bytes(), []byte("fatal: reference is not a tree: ")) {
			panic(integrity.DataDNE.New("hash %q not found in this repo", dataHash))
		}
		if p.GetExitCode() != 0 {
			// catchall.
			// this formatting is *terrible*, but we don't have a good formatter for using datakeys, either, so.
			// (blowing past this without too much fuss because we're going to switch error libraries later and it's going to fix this better.)
			panic(Error.New("git checkout failed.  git output:\n%s", buf.String()))
		}
		log.Info("git transmat: checkout complete")
		// And, do submodules.
		git.Bake(
			"submodule", "update", "--init",
			gosh.Opts{Cwd: arena.workDirPath},
		).RunAndReport()
		log.Info("git transmat: submodules complete")

		// verify total integrity
		// actually this is a nil step; there's no such thing as "acceptHashMismatch", clone would have simply failed
		arena.hash = dataHash
	}).Catch(integrity.Error, func(err *errors.Error) {
		panic(err)
	}).CatchAll(func(err error) {
		panic(integrity.UnknownError.Wrap(err))
	}).Done()
	return arena
}

func (t GitTransmat) Scan(
	kind integrity.TransmatKind,
	subjectPath string,
	siloURIs []integrity.SiloURI,
	log log15.Logger,
	options ...integrity.MaterializerConfigurer,
) integrity.CommitID {
	var commitID integrity.CommitID
	try.Do(func() {
		// Basic validation and config
		//config := integrity.EvaluateConfig(options...)
		if kind != Kind {
			panic(errors.ProgrammerError.New("This transmat supports definitions of type %q, not %q", Kind, kind))
		}

		// Get off my lawn.
		panic(errors.NotImplementedError.New("The git transmat does not support scan."))
	}).Catch(integrity.Error, func(err *errors.Error) {
		panic(err)
	}).CatchAll(func(err error) {
		panic(integrity.UnknownError.Wrap(err))
	}).Done()
	return commitID
}

type gitArena struct {
	gitDirPath  string
	workDirPath string
	hash        integrity.CommitID
}

func (a gitArena) Path() string {
	return a.workDirPath
}

func (a gitArena) Hash() integrity.CommitID {
	return a.hash
}

// rm's.
// does not consider it an error if path already does not exist.
func (a gitArena) Teardown() {
	if err := os.RemoveAll(a.workDirPath); err != nil {
		if e2, ok := err.(*os.PathError); !ok || e2.Err != syscall.ENOENT || e2.Path != a.workDirPath {
			panic(err)
		}
	}
	if err := os.RemoveAll(a.gitDirPath); err != nil {
		if e2, ok := err.(*os.PathError); !ok || e2.Err != syscall.ENOENT || e2.Path != a.gitDirPath {
			panic(err)
		}
	}
}
