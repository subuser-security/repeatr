package runc

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"os/exec"
	"path/filepath"

	"github.com/inconshreveable/log15"
	"github.com/polydawn/gosh"
	"github.com/spacemonkeygo/errors"
	"github.com/spacemonkeygo/errors/try"

	"polydawn.net/repeatr/def"
	"polydawn.net/repeatr/executor"
	"polydawn.net/repeatr/executor/basicjob"
	"polydawn.net/repeatr/executor/util"
	"polydawn.net/repeatr/io"
	"polydawn.net/repeatr/lib/flak"
	"polydawn.net/repeatr/lib/streamer"
)

// interface assertion
var _ executor.Executor = &Executor{}

type Executor struct {
	workspacePath string
}

func (e *Executor) Configure(workspacePath string) {
	e.workspacePath = workspacePath
}

func (e *Executor) Start(f def.Formula, id def.JobID, stdin io.Reader, journal io.Writer) def.Job {

	// Prepare the forumla for execution on this host
	def.ValidateAll(&f)

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

			// Set up a logger.  Tag all messages with this jobid.
			logger := log15.New(log15.Ctx{"JobID": id})
			logger.SetHandler(log15.StreamHandler(journal, log15.TerminalFormat()))

			job.Result = e.Run(f, job, dir, stdin, outS, errS, logger)
		}, e.workspacePath, "job", string(job.Id()))

		// Directory is clean; job complete
		close(job.WaitChan)
	}()

	<-jobReady
	return job
}

// Executes a job, catching any panics.
func (e *Executor) Run(f def.Formula, j def.Job, d string, stdin io.Reader, outS, errS io.WriteCloser, journal log15.Logger) def.JobResult {
	r := def.JobResult{
		ID:       j.Id(),
		ExitCode: -1,
	}

	try.Do(func() {
		e.Execute(f, j, d, &r, outS, errS, journal)
	}).Catch(executor.Error, func(err *errors.Error) {
		r.Error = err
	}).Catch(integrity.Error, func(err *errors.Error) {
		r.Error = err
	}).CatchAll(func(err error) {
		r.Error = executor.UnknownError.Wrap(err).(*errors.Error)
	}).Done()

	return r
}

// Execute a formula in a specified directory. MAY PANIC.
func (e *Executor) Execute(formula def.Formula, job def.Job, jobPath string, result *def.JobResult, stdout, stderr io.WriteCloser, journal log15.Logger) {
	rootfsPath := filepath.Join(jobPath, "rootfs")
	logPath := filepath.Join(jobPath, "runc-debug.log")

	// Prepare inputs
	transmat := util.DefaultTransmat()
	inputArenas := util.ProvisionInputs(transmat, formula.Inputs, journal)
	util.ProvisionOutputs(formula.Outputs, rootfsPath, journal)

	// Assemble filesystem
	assembly := util.AssembleFilesystem(
		util.BestAssembler(),
		rootfsPath,
		formula.Inputs,
		inputArenas,
		formula.Action.Escapes.Mounts,
		journal,
	)
	defer assembly.Teardown()

	// Emit configs for runc.
	runcConfigJsonPath := filepath.Join(jobPath, "config.json")
	cfg := EmitRuncConfigStruct(formula)
	buf, err := json.Marshal(cfg)
	if err != nil {
		panic(executor.UnknownError.Wrap(err))
	}
	ioutil.WriteFile(runcConfigJsonPath, buf, 0600)
	runcRuntimeJsonPath := filepath.Join(jobPath, "runtime.json")
	cfg = EmitRuncRuntimeStruct(formula)
	buf, err = json.Marshal(cfg)
	if err != nil {
		panic(executor.UnknownError.Wrap(err))
	}
	ioutil.WriteFile(runcRuntimeJsonPath, buf, 0600)

	// Prepare command to exec
	args := []string{
		"--root", filepath.Join(e.workspacePath, "shared"), // a tmpfs would be appropriate
		"--log", logPath,
		"start",
		"--config-file", runcConfigJsonPath,
		"--runtime-file", runcRuntimeJsonPath,
	}
	cmd := exec.Command("runc", args...)
	cmd.Stdin = nil
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	// launch execution.
	// transform gosh's typed errors to repeatr's hierarchical errors.
	// this is... not untroubled code: since we're invoking a helper that's then
	//  proxying the exec even further, most errors are fatal (the mapping here is
	//   very different than in e.g. chroot executor, and provides much less meaning).
	var proc gosh.Proc
	try.Do(func() {
		proc = gosh.ExecProcCmd(cmd)
	}).CatchAll(func(err error) {
		switch err.(type) {
		case gosh.NoSuchCommandError:
			panic(executor.ConfigError.New("runc binary is missing"))
		case gosh.NoArgumentsError:
			panic(executor.UnknownError.Wrap(err))
		case gosh.NoSuchCwdError:
			panic(executor.UnknownError.Wrap(err))
		case gosh.ProcMonitorError:
			panic(executor.TaskExecError.Wrap(err))
		default:
			panic(executor.UnknownError.Wrap(err))
		}
	}).Done()

	// Wait for the job to complete
	result.ExitCode = proc.GetExitCode()

	// Save outputs
	result.Outputs = util.PreserveOutputs(transmat, formula.Outputs, rootfsPath, journal)

	// TODO : additional error detection of runc failure modes
}
