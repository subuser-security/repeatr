package cli

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"polydawn.net/repeatr/testutil"
)

var (
	// os flag parsing mandates the executable name
	baseArgs = []string{"repeatr"}
)

// These can be swapped wholesale; demonstration only.

func Test(t *testing.T) {

	Convey("It should not crash without args", t, func() {
		App.Run(baseArgs)
	})

	testutil.Convey_IfCanNS("Within an environment that can run namespaces", t, func() {
		testutil.Convey_IfHaveRoot("It should run a basic example", func() {
			App.Run(append(baseArgs, "run", "-i", "../lib/integration/basic.json"))
		})
	})

}
