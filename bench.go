package starlarkassert

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"go.starlark.net/starlark"
)

// Bench is passed to starlark benchmark functions.
// Interface is based on Go's *testing.B.
//
//   def bench_bar(b):
//      for _ in range(b.n):
//         ...work...
//
type Bench struct {
	b *testing.B
}

func NewBench(b *testing.B) *Bench {
	return &Bench{b: b}
}

func (*Bench) Freeze()               {}
func (*Bench) Truth() starlark.Bool  { return true }
func (*Bench) Type() string          { return "benchmark" }
func (*Bench) String() string        { return "<benchmark>" }
func (*Bench) Hash() (uint32, error) { return 0, fmt.Errorf("unhashable: benchmark") }
func (b *Bench) Attr(name string) (starlark.Value, error) {
	if m := benchAttrs[name]; m != nil {
		return m(b), nil
	}
	return nil, nil
}
func (*Bench) AttrNames() []string {
	names := make([]string, 0, len(benchAttrs))
	for name := range benchAttrs {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

type benchAttr func(b *Bench) starlark.Value

var benchAttrs = map[string]benchAttr{
	"restart": func(b *Bench) starlark.Value { return method{b, "restart", b.restart} },
	"start":   func(b *Bench) starlark.Value { return method{b, "start", b.start} },
	"stop":    func(b *Bench) starlark.Value { return method{b, "stop", b.stop} },
	"n":       func(b *Bench) starlark.Value { return starlark.MakeInt(b.b.N) },

	"error":  func(b *Bench) starlark.Value { return tmethod{b, "error", b.b, terror} },
	"fail":   func(b *Bench) starlark.Value { return tmethod{b, "fail", b.b, tfail} },
	"fatal":  func(b *Bench) starlark.Value { return tmethod{b, "fatal", b.b, tfatal} },
	"freeze": func(b *Bench) starlark.Value { return method{b, "freeze", freeze} },
	"skip":   func(b *Bench) starlark.Value { return tmethod{b, "skip", b.b, tskip} },

	"eq":        func(b *Bench) starlark.Value { return tmethod{b, "eq", b.b, teq} },
	"equal":     func(b *Bench) starlark.Value { return tmethod{b, "eq", b.b, teq} },
	"ne":        func(b *Bench) starlark.Value { return tmethod{b, "ne", b.b, tne} },
	"not_equal": func(b *Bench) starlark.Value { return tmethod{b, "ne", b.b, tne} },
	"true":      func(b *Bench) starlark.Value { return tmethod{b, "true", b.b, ttrue} },
	"lt":        func(b *Bench) starlark.Value { return tmethod{b, "lt", b.b, tlt} },
	"less_than": func(b *Bench) starlark.Value { return tmethod{b, "lt", b.b, tlt} },
	"contains":  func(b *Bench) starlark.Value { return tmethod{b, "contains", b.b, tcontains} },
	"fails":     func(b *Bench) starlark.Value { return tmethod{b, "fails", b.b, tfails} },
}

func (b *Bench) restart(_ *starlark.Thread, _ starlark.Tuple, _ []starlark.Tuple) (starlark.Value, error) {
	b.b.ResetTimer()
	return starlark.None, nil
}

func (b *Bench) start(_ *starlark.Thread, _ starlark.Tuple, _ []starlark.Tuple) (starlark.Value, error) {
	b.b.StartTimer()
	return starlark.None, nil
}

func (b *Bench) stop(_ *starlark.Thread, _ starlark.Tuple, _ []starlark.Tuple) (starlark.Value, error) {
	b.b.StopTimer()
	return starlark.None, nil
}

// BenchFile runs each function with the prefix "bench_" as a b.Run func.
func BenchFile(b *testing.B, filename string, src interface{}, globals starlark.StringDict, opts ...TestOption) {
	b.Helper()

	thread, cleanup := newThread(b, filename, opts)
	b.Cleanup(cleanup)

	values, err := starlark.ExecFile(thread, filename, src, globals)
	if err != nil {
		errorf(b, filename, err)
		return
	}

	for key, val := range values {
		if !strings.HasPrefix(key, "bench_") {
			continue // ignore
		}
		if _, ok := val.(starlark.Callable); !ok {
			continue // ignore non callable
		}

		key, val := key, val
		b.Run(key, func(b *testing.B) {

			bb := NewBench(b)
			name := thread.Name
			thread, cleanup := newThread(b, name, opts)
			defer cleanup()

			if _, err := starlark.Call(
				thread, val, starlark.Tuple{bb}, nil,
			); err != nil {
				errorf(b, name, err)
			}
		})
	}

}

// RunBenches is a local bench suite runner. Each file in the pattern glob is ran.
// To use add it to a Benchmark function:
//
// 	func BenchmarkStarlark(b *testing.B) {
// 		globals := starlark.StringDict{}
// 		RunBenches(b, "testdata/*.star", globals)
// 	}
//
func RunBenches(b *testing.B, pattern string, globals starlark.StringDict, opts ...TestOption) {
	b.Helper()

	files, err := filepath.Glob(pattern)
	if err != nil {
		b.Fatal(err)
	}

	for _, filename := range files {
		BenchFile(b, filename, nil, globals, opts...)
	}
}
