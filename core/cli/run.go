package cli

import (
	"fmt"
	"io"
	"io/ioutil"
	"path/filepath"

	"github.com/inconshreveable/log15"

	"go.polydawn.net/repeatr/api/def"
	"go.polydawn.net/repeatr/api/hitch"
	"go.polydawn.net/repeatr/core/actors"
	"go.polydawn.net/repeatr/core/executor"
)

func LoadFormulaFromFile(path string) def.Formula {
	filename, _ := filepath.Abs(path)

	content, err := ioutil.ReadFile(filename)
	if err != nil {
		panic(Error.Wrap(fmt.Errorf("Could not read formula file %q: %s", filename, err)))
	}

	// TODO eagerly await the day we switch this out to full Meep and this becomes not-ugly again
	defer func() {
		complaint := recover()
		if complaint != nil {
			panic(Error.Wrap(complaint.(error)))
		}
	}()

	return *hitch.ParseYaml(content)
}

func RunFormula(execr executor.Executor, formula def.Formula, journal io.Writer) *def.RunRecord {
	// Set up a logger.
	log := log15.New()
	log.SetHandler(log15.StreamHandler(journal, log15.TerminalFormat()))

	// Create a local formula runner, and fire.
	runner := actor.NewFormulaRunner(execr, log)
	runID := runner.StartRun(&formula)

	// Stream job output to terminal in real time
	//  (stderr and stdout of the job both go to the same stream as our own logs).
	runner.FollowStreams(runID, journal, journal)

	// Wait for results.
	result := runner.FollowResults(runID)
	if result.Failure == nil {
		log.Info("Job finished",
			"outputs", result.Results,
		)
	} else {
		log.Error("Job execution errored",
			"error", result.Failure,
		)
	}
	return result
}
