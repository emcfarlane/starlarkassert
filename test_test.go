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
	opt := func(_ testing.TB, thread *starlark.Thread) func() {
		originalLoad := thread.Load
		thread.Load = func(thread *starlark.Thread, module string) (starlark.StringDict, error) {
			switch module {
			case "test_load.star":
				return starlark.StringDict{
					"greet": starlark.String("world"),
				}, nil
			}
			return originalLoad(thread, module)
		}
		return func() {
			thread.Load = originalLoad
		}
	}

	RunTests(t, "testdata/*.star", globals, opt)
}
