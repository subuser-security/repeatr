package chroot

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/inconshreveable/log15"
	"github.com/polydawn/gosh"
	"go.polydawn.net/meep"

	"go.polydawn.net/repeatr/api/def"
	"go.polydawn.net/repeatr/core/executor"
	"go.polydawn.net/repeatr/core/executor/basicjob"
	"go.polydawn.net/repeatr/core/executor/cradle"
	"go.polydawn.net/repeatr/core/executor/util"
	"go.polydawn.net/repeatr/lib/flak"
	"go.polydawn.net/repeatr/lib/streamer"
)

var _ executor.Executor = &Executor{} // interface assertion

type Executor struct {
	workspacePath string
}

func (e *Executor) Configure(workspacePath string) {
	e.workspacePath = workspacePath
}

func (e *Executor) Start(f def.Formula, id executor.JobID, stdin io.Reader, log log15.Logger) executor.Job {
	// Fill in default config for anything still blank.
	f = *cradle.ApplyDefaults(&f)

	job := basicjob.New(id)
	jobReady := make(chan struct{})

	go func() {
		// Run the formula in a temporary directory
		flak.WithDir(func(dir string) {

			// spool our output to a muxed stream
			var strm streamer.Mux
			strm = streamer.CborFileMux(filepath.Join(dir, "log"))
			outS := strm.Appender(1)
			errS := strm.Appender(2)
			job.Streams = strm
			defer func() {
				// Regardless of how the job ends (or even if it fails the remaining setup), output streams must be terminated.
				outS.Close()
				errS.Close()
			}()

			// Job is ready to stream process output
			close(jobReady)

			job.Result = e.Run(f, job, dir, stdin, outS, errS, log)
		}, e.workspacePath, "job", string(job.Id()))

		// Directory is clean; job complete
		close(job.WaitChan)
	}()

	<-jobReady
	return job
}

// Executes a job, catching any panics.
func (e *Executor) Run(f def.Formula, j executor.Job, d string, stdin io.Reader, outS, errS io.WriteCloser, journal log15.Logger) executor.JobResult {
	r := executor.JobResult{
		ID:       j.Id(),
		ExitCode: -1,
	}

	r.Error = meep.RecoverPanics(func() {
		e.Execute(f, j, d, &r, stdin, outS, errS, journal)
	})
	return r
}

// Execute a formula in a specified directory. MAY PANIC.
func (e *Executor) Execute(f def.Formula, j executor.Job, d string, result *executor.JobResult, stdin io.Reader, outS, errS io.WriteCloser, journal log15.Logger) {
	// Prepare inputs
	transmat := util.DefaultTransmat()
	inputArenas := util.ProvisionInputs(transmat, f.Inputs, journal)

	// Assemble filesystem
	rootfs := filepath.Join(d, "rootfs")
	assembly := util.AssembleFilesystem(
		util.BestAssembler(),
		rootfs,
		f.Inputs,
		inputArenas,
		f.Action.Escapes.Mounts,
		journal,
	)
	defer assembly.Teardown() // What ever happens: Disassemble filesystem
	util.ProvisionOutputs(f.Outputs, rootfs, journal)
	if f.Action.Cradle == nil || *(f.Action.Cradle) == true {
		cradle.MakeCradle(rootfs, f)
	}

	// chroot's are pretty easy.
	cmdName := f.Action.Entrypoint[0]
	cmd := exec.Command(cmdName, f.Action.Entrypoint[1:]...)
	userinfo := cradle.UserinfoForPolicy(f.Action.Policy)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Chroot: rootfs,
		Credential: &syscall.Credential{
			Uid: uint32(userinfo.Uid),
			Gid: uint32(userinfo.Gid),
		},
	}

	// except handling cwd is a little odd.
	// see comments in gosh tests with chroot for information about the odd behavior we're hacking around here;
	// we're comfortable making this special check here, but not upstreaming it to gosh, because in our context we "know" we're not racing anyone.
	if externalCwdStat, err := os.Stat(filepath.Join(rootfs, f.Action.Cwd)); err != nil {
		panic(executor.NoSuchCwdError.New("cannot set cwd to %q: %s", f.Action.Cwd, err.(*os.PathError).Err))
	} else if !externalCwdStat.IsDir() {
		panic(executor.NoSuchCwdError.New("cannot set cwd to %q: not a directory", f.Action.Cwd))
	}
	cmd.Dir = f.Action.Cwd

	// set env.
	// initialization already required by earlier 'validate' calls.
	cmd.Env = envToSlice(f.Action.Env)

	cmd.Stdin = stdin
	cmd.Stdout = outS
	cmd.Stderr = errS

	// launch execution.
	// transform gosh's typed errors to repeatr's hierarchical errors.
	startedExec := time.Now()
	journal.Info("Beginning execution!")
	var proc gosh.Proc
	meep.Try(func() {
		proc = gosh.ExecProcCmd(cmd)
	}, meep.TryPlan{
		{ByType: gosh.NoSuchCommandError{}, Handler: func(err error) {
			panic(executor.NoSuchCommandError.Wrap(err))
		}},
		{ByType: gosh.NoArgumentsError{}, Handler: func(err error) {
			panic(executor.NoSuchCommandError.Wrap(err))
		}},
		{ByType: gosh.NoSuchCwdError{}, Handler: func(err error) {
			// included for clarity and completeness, but we'll never actually see this; see comments in gosh about the interaction of chroot and cwd error handling.
			panic(executor.TaskExecError.Wrap(err))
		}},
		{ByType: gosh.ProcMonitorError{}, Handler: func(err error) {
			panic(executor.TaskExecError.Wrap(err))
		}},
		{CatchAny: true, Handler: func(err error) {
			panic(executor.UnknownError.Wrap(err))
		}},
	})

	// Wait for the job to complete
	// REVIEW: consider exposing `gosh.Proc`'s interface as part of repeatr's job tracking api?
	result.ExitCode = proc.GetExitCode()
	journal.Info("Execution done!",
		"elapsed", time.Now().Sub(startedExec).Seconds(),
	)

	// Save outputs
	result.Outputs = util.PreserveOutputs(transmat, f.Outputs, rootfs, journal)
}

func envToSlice(env map[string]string) []string {
	rv := make([]string, len(env))
	i := 0
	for k, v := range env {
		rv[i] = k + "=" + v
		i++
	}
	return rv
}
