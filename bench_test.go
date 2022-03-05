package starlarkassert

import (
	"testing"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

func BenchmarkRunBenches(b *testing.B) {
	globals := starlark.StringDict{
		"struct": starlark.NewBuiltin("struct", starlarkstruct.Make),
	}
	RunBenches(b, "testdata/bench.star", globals)
}
