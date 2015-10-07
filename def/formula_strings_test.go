package def_test

import (
	"testing"

	"github.com/go-yaml/yaml"
	. "github.com/smartystreets/goconvey/convey"

	"polydawn.net/repeatr/def"
	"polydawn.net/repeatr/lib/cereal"
)

func TestStringParse(t *testing.T) {
	Convey("Given a basic formula", t, func() {
		content := []byte(`
		inputs:
			"/":
				type: "bonk"
				hash: "asdf"
		action:
			command:
				- "shellit"
		outputs:
			"/dev/null":
				type: "nope"
		`)

		Convey("It should parse", func() {
			content = cereal.Tab2space(content)
			var tree interface{}
			if err := yaml.Unmarshal(content, &tree); err != nil {
				panic(err)
			}
			tree = cereal.StringifyMapKeys(tree)

			formula := &def.Formula{}
			err := formula.Unmarshal(tree)
			So(err, ShouldBeNil)
			So(len(formula.Inputs), ShouldEqual, 1)
			So(formula.Inputs[0].MountPath, ShouldEqual, "/")
			So(formula.Inputs[0].Hash, ShouldEqual, "asdf")
		})
	})
}
