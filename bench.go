// Package starlarkassert is an extension of go.starlark.net/starlarktest
// to integrate into go's testing pacakge.
package starlarkassert

import (
	"fmt"
	"path"
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

type method struct {
	recv starlark.Value
	name string
	fn   func(*starlark.Thread, starlark.Tuple, []starlark.Tuple) (starlark.Value, error)
}

func (m method) Name() string          { return m.name }
func (m method) Freeze()               {}
func (m method) Hash() (uint32, error) { return 0, nil }
func (m method) String() string {
	return fmt.Sprintf("<builtin_method %s of %s value>", m.Name(), m.recv.Type())
}
func (m method) Type() string         { return "builtin_method" }
func (m method) Truth() starlark.Bool { return true }
func (m method) CallInternal(thread *starlark.Thread, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return m.fn(thread, args, kwargs)
}

type benchAttr func(b *Bench) starlark.Value

var benchAttrs = map[string]benchAttr{
	"restart": func(b *Bench) starlark.Value { return method{b, "restart", b.restart} },
	"start":   func(b *Bench) starlark.Value { return method{b, "start", b.start} },
	"stop":    func(b *Bench) starlark.Value { return method{b, "stop", b.stop} },
	"n":       func(b *Bench) starlark.Value { return starlark.MakeInt(b.b.N) },
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
	defer cleanup()

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
			name := path.Join(filename, key)
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
// 	func BenchmarkStarlark(t *testing.T) {
// 		globals := starlark.StringDict{}
// 		RunBenches(t, "testdata/*_bench.star", globals)
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