package starlarkassert

import (
	"testing"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

func TestRunTests(t *testing.T) {
	globals := starlark.StringDict{
		"struct": starlark.NewBuiltin("struct", starlarkstruct.Make),
	}
	opt := WithLoad(func(_ *starlark.Thread, module string) (starlark.StringDict, error) {
		switch module {
		case "test_load.star":
			return starlark.StringDict{
				"greet": starlark.String("world"),
			}, nil
		default:
			return nil, nil
		}
	})
	RunTests(t, "testdata/*.star", globals, opt)
}

func Test_depsInterface(t *testing.T) {
	t.Skip() // Just check it compiles
	var deps MatchStringOnly = nil
	testing.MainStart(deps, nil, nil, nil, nil)
}
