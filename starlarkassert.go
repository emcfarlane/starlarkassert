// Package starlarkassert is an extension of go.starlark.net/starlarktest
// to integrate into go's testing pacakge.
package starlarkassert

import (
	_ "embed"
	"fmt"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"go.starlark.net/starlark"
)

// Testing is passed to starlark functions.
type Testing struct {
	t      *testing.T
	frozen bool
}

func NewTesting(t *testing.T) *Testing {
	return &Testing{t: t}
}

func (t *Testing) String() string        { return "<testing>" }
func (t *Testing) Type() string          { return "testing.t" }
func (t *Testing) Freeze()               { t.frozen = true }
func (t *Testing) Truth() starlark.Bool  { return t.t != nil }
func (t *Testing) Hash() (uint32, error) { return 0, fmt.Errorf("unhashable type: %s", t.Type()) }

var testingMethods = map[string]*starlark.Builtin{
	"run":  starlark.NewBuiltin("testing.t.run", testingRun),
	"skip": starlark.NewBuiltin("testing.t.skip", testingSkip),
	"fail": starlark.NewBuiltin("testing.t.fail", testingFail),
}

func (t *Testing) Attr(name string) (starlark.Value, error) {
	m := testingMethods[name]
	if m == nil {
		return nil, nil
	}
	return m.BindReceiver(t), nil
}
func (t *Testing) AttrNames() []string {
	names := make([]string, 0, len(testingMethods))
	for name := range testingMethods {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func wrapTLog(t *testing.T, thread *starlark.Thread) func() {
	reporter := GetReporter(thread)
	SetReporter(thread, t)
	print := thread.Print
	thread.Print = func(_ *starlark.Thread, s string) { t.Log(s) }
	return func() {
		SetReporter(thread, reporter)
		thread.Print = print
	}
}

func testingRun(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	t := b.Receiver().(*Testing)
	if t.frozen {
		return nil, fmt.Errorf("testing.t: frozen")
	}

	var (
		name string
		fn   starlark.Callable
	)
	if err := starlark.UnpackArgs(
		"testing.run", args, kwargs, "name", &name, "fn", &fn,
	); err != nil {
		return nil, err
	}

	var (
		val starlark.Value
		err error
	)
	t.t.Run(name, func(t *testing.T) {
		tval := NewTesting(t)
		defer wrapTLog(t, thread)()
		val, err = starlark.Call(thread, fn, starlark.Tuple{tval}, nil)
	})
	return val, err
}

func testingSkip(_ *starlark.Thread, b *starlark.Builtin, _ starlark.Tuple, _ []starlark.Tuple) (starlark.Value, error) {
	t := b.Receiver().(*Testing)
	t.t.Skip() // TODO: printing
	return starlark.None, nil
}

func testingFail(_ *starlark.Thread, b *starlark.Builtin, _ starlark.Tuple, _ []starlark.Tuple) (starlark.Value, error) {
	t := b.Receiver().(*Testing)
	t.t.Fail()
	return starlark.None, nil
}

func errorf(t testing.TB, filename string, err error) {
	t.Helper()

	switch err := err.(type) {
	case *starlark.EvalError:
		var found bool
		for i := range err.CallStack {
			posn := err.CallStack.At(i).Pos
			if posn.Filename() == filename {
				linenum := int(posn.Line)
				msg := err.Error()

				t.Errorf("\n%s:%d: unexpected error: %v", filename, linenum, msg)
				found = true
				break
			}
		}
		if !found {
			t.Error(err.Backtrace())
		}
	case nil:
		// success
	default:
		t.Errorf("\n%s", err)
	}
}

func newThread(t testing.TB, name string, opts []TestOption) (*starlark.Thread, func()) {
	thread := &starlark.Thread{Name: name}

	var cleanups []func()
	for _, opt := range opts {
		if v := opt(t, thread); v != nil {
			cleanups = append(cleanups, v)
		}
	}

	SetReporter(thread, t)
	thread.Print = func(_ *starlark.Thread, msg string) {
		t.Log(msg)
	}
	load := thread.Load
	thread.Load = func(thread *starlark.Thread, module string) (starlark.StringDict, error) {
		if module == "assert.star" {
			return LoadAssertModule(thread)
		}
		if load != nil {
			return load(thread, module)
		}
		return nil, nil
	}
	return thread, func() {
		for _, cleanup := range cleanups {
			cleanup()
		}
	}
}

type TestOption func(t testing.TB, thread *starlark.Thread) func()

// TestFile runs each function with the prefix "test_" in parallel as a t.Run func.
func TestFile(t *testing.T, filename string, src interface{}, globals starlark.StringDict, opts ...TestOption) {
	t.Helper()

	thread, cleanup := newThread(t, filename, opts)
	defer cleanup()

	values, err := starlark.ExecFile(thread, filename, src, globals)
	if err != nil {
		errorf(t, filename, err)
		return
	}

	for key, val := range values {
		if !strings.HasPrefix(key, "test_") {
			continue // ignore
		}
		if _, ok := val.(starlark.Callable); !ok {
			continue // ignore non callable
		}

		key, val := key, val
		t.Run(key, func(t *testing.T) {
			t.Parallel()

			tt := NewTesting(t)
			name := path.Join(filename + "/" + key)
			thread, cleanup := newThread(t, name, opts)
			defer cleanup()

			if _, err := starlark.Call(
				thread, val, starlark.Tuple{tt}, nil,
			); err != nil {
				errorf(t, name, err)
			}
		})
	}
}

// RunTests is a local test suite runner. Each file in the pattern glob is ran.
// To use add it to a Test function:
//
// func TestRun(t *testing.T) {
// 	globals := starlark.StringDict{}
// 	RunTests(t, "testdata/*_test.star", globals)
// }
//
func RunTests(t *testing.T, pattern string, globals starlark.StringDict, opts ...TestOption) {
	t.Helper()

	files, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatal(err)
	}

	for _, filename := range files {
		TestFile(t, filename, nil, globals, opts...)
	}
}
