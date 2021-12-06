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
	runner := func(thread *starlark.Thread, handler func() error) error {
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
		return handler()
	}

	RunTests(t, "testdata/*.star", globals, runner)
}
