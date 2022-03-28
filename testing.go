package starlarkassert

import (
	"errors"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

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
	"fail": func(t *Test) starlark.Value { return method{t, "fail", t.fail} },
	"run":  func(t *Test) starlark.Value { return method{t, "run", t.run} },
	"skip": func(t *Test) starlark.Value { return method{t, "skip", t.skip} },
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

	reporter := GetReporter(thread)
	SetReporter(thread, t)
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
	return func() {
		SetReporter(thread, reporter)
		thread.Print = print
	}
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

func (t *Test) skip(_ *starlark.Thread, _ starlark.Tuple, _ []starlark.Tuple) (starlark.Value, error) {
	t.t.Skip() // TODO: printing
	return starlark.None, nil
}

func (t *Test) fail(_ *starlark.Thread, _ starlark.Tuple, _ []starlark.Tuple) (starlark.Value, error) {
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
	cleanups = append(cleanups, wrapLog(t, thread))
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

// TestOption is called on setup with an optional cleanup func called on teardown.
type TestOption func(t testing.TB, thread *starlark.Thread) func()

func WithModule(name string, module starlark.StringDict) TestOption {
	return func(_ testing.TB, thread *starlark.Thread) func() {
		load := thread.Load
		thread.Load = func(thread *starlark.Thread, loadName string) (starlark.StringDict, error) {
			if loadName == name {
				return module, nil
			}
			if load != nil {
				return load(thread, loadName)
			}
			return nil, nil
		}
		return nil
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

type corpusEntry = struct {
	Parent     string
	Path       string
	Data       []byte
	Values     []any
	Generation int
	IsSeed     bool
}

// No one should be using func Main anymore.
// See the doc comment on func Main and use MainStart instead.
var errMain = errors.New("testing: unexpected use of func Main")

// MatchStringOnly is an implementation of the internal testing.testDeps interface.
// Interface is unstable and likely to break in new go versions. Current go 1.18.
type MatchStringOnly func(pat, str string) (bool, error)

func (f MatchStringOnly) MatchString(pat, str string) (bool, error)   { return f(pat, str) }
func (f MatchStringOnly) StartCPUProfile(w io.Writer) error           { return errMain }
func (f MatchStringOnly) StopCPUProfile()                             {}
func (f MatchStringOnly) WriteProfileTo(string, io.Writer, int) error { return errMain }
func (f MatchStringOnly) ImportPath() string                          { return "" }
func (f MatchStringOnly) StartTestLog(io.Writer)                      {}
func (f MatchStringOnly) StopTestLog() error                          { return errMain }
func (f MatchStringOnly) SetPanicOnExit0(bool)                        {}
func (f MatchStringOnly) CoordinateFuzzing(time.Duration, int64, time.Duration, int64, int, []corpusEntry, []reflect.Type, string, string) error {
	return errMain
}
func (f MatchStringOnly) RunFuzzWorker(func(corpusEntry) error) error { return errMain }
func (f MatchStringOnly) ReadCorpus(string, []reflect.Type) ([]corpusEntry, error) {
	return nil, errMain
}
func (f MatchStringOnly) CheckCorpus([]any, []reflect.Type) error { return nil }
func (f MatchStringOnly) ResetCoverage()                          {}
func (f MatchStringOnly) SnapshotCoverage()                       {}
