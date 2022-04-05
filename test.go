package starlarkassert

import (
	"fmt"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"

	"go.starlark.net/starlark"
)

// Test is passed to starlark testing functions.
// Interface is based on Go's *testing.T.
//
// 	def test_foo(t):
// 	    ...check...
//
type Test struct {
	t      *testing.T
	frozen bool
}

func NewTest(t *testing.T) *Test {
	return &Test{t: t}
}

func (t *Test) String() string        { return "<test>" }
func (t *Test) Type() string          { return "test" }
func (t *Test) Freeze()               { t.frozen = true }
func (t *Test) Truth() starlark.Bool  { return t.t != nil }
func (t *Test) Hash() (uint32, error) { return 0, fmt.Errorf("unhashable type: %s", t.Type()) }

type testAttr func(t *Test) starlark.Value

var testAttrs = map[string]testAttr{
	"error":  func(t *Test) starlark.Value { return tmethod{t, "error", t.t, terror} },
	"fail":   func(t *Test) starlark.Value { return tmethod{t, "fail", t.t, tfail} },
	"fatal":  func(t *Test) starlark.Value { return tmethod{t, "fatal", t.t, tfatal} },
	"freeze": func(t *Test) starlark.Value { return method{t, "freeze", freeze} },
	"run":    func(t *Test) starlark.Value { return method{t, "run", t.run} },
	"skip":   func(t *Test) starlark.Value { return tmethod{t, "skip", t.t, tskip} },

	"eq":       func(t *Test) starlark.Value { return tmethod{t, "eq", t.t, teq} },
	"ne":       func(t *Test) starlark.Value { return tmethod{t, "ne", t.t, tne} },
	"true":     func(t *Test) starlark.Value { return tmethod{t, "true", t.t, ttrue} },
	"lt":       func(t *Test) starlark.Value { return tmethod{t, "lt", t.t, tlt} },
	"contains": func(t *Test) starlark.Value { return tmethod{t, "contains", t.t, tcontains} },
	"fails":    func(t *Test) starlark.Value { return tmethod{t, "fails", t.t, tfails} },
}

func (t *Test) Attr(name string) (starlark.Value, error) {
	if m := testAttrs[name]; m != nil {
		return m(t), nil
	}
	return nil, nil
}
func (t *Test) AttrNames() []string {
	names := make([]string, 0, len(testAttrs))
	for name := range testAttrs {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func wrapLog(t testing.TB, thread *starlark.Thread) func() {
	_, origFile, origLine, _ := runtime.Caller(0)

	print := thread.Print
	thread.Print = func(thread *starlark.Thread, s string) {
		cf := thread.CallFrame(1)
		s = fmt.Sprintf("%s:%d:%d %s", thread.Name, cf.Pos.Line, cf.Pos.Col, s)

		// Overwrite go's filename in log.
		erase := strings.Repeat("\b", len(path.Base(origFile))+len(strconv.Itoa(origLine))+3)
		if diff := len(erase) - len(s); diff > 0 {
			s += strings.Repeat(" ", diff)
		}
		t.Logf("%s%s", erase, s)
	}
	return func() { thread.Print = print }
}

func (t *Test) run(thread *starlark.Thread, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
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
		defer wrapLog(t, thread)()

		tval := NewTest(t)
		val, err = starlark.Call(thread, fn, starlark.Tuple{tval}, nil)
		if err != nil {
			t.Error(err)
		}
	})
	if err != nil {
		return starlark.None, nil
	}
	return val, nil
}

func (t *Test) fatal(thread *starlark.Thread, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	s, err := pprint(thread, args, kwargs)
	if err != nil {
		t.t.Fatal(err)
	}
	t.t.Fatal(s)
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
	cleanups = append(cleanups, wrapLog(t, thread))
	return thread, func() {
		for _, cleanup := range cleanups {
			cleanup()
		}
	}
}

// TestOption is called on setup with an optional cleanup func called on teardown.
type TestOption func(t testing.TB, thread *starlark.Thread) func()

// WithLoad adds a loader to the thread. If the loader returns nil, the previous
// loader will be called.
func WithLoad(load func(*starlark.Thread, string) (starlark.StringDict, error)) TestOption {
	return func(_ testing.TB, thread *starlark.Thread) func() {
		oldLoad := thread.Load
		thread.Load = func(thread *starlark.Thread, loadName string) (starlark.StringDict, error) {
			m, err := load(thread, loadName)
			if m != nil || err != nil {
				return m, err
			}
			if oldLoad != nil {
				return oldLoad(thread, loadName)
			}
			return nil, nil
		}
		return func() { thread.Load = oldLoad }
	}
}

// TestFile runs each function with the prefix "test_" in parallel as a t.Run func.
func TestFile(t *testing.T, filename string, src interface{}, globals starlark.StringDict, opts ...TestOption) {
	t.Helper()

	thread, cleanup := newThread(t, filename, opts)
	t.Cleanup(cleanup)

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

			tt := NewTest(t)
			name := thread.Name
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
// 	func TestStarlark(t *testing.T) {
// 		globals := starlark.StringDict{}
// 		RunTests(t, "testdata/*_test.star", globals)
// 	}
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
