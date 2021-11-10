// Copyright 2017 The Bazel Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package starlarkassert is based on go.starlark.net/starlarktest but modified
// to embedded starlark files for go modules support.
package starlarkassert

import (
	_ "embed"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

const localKey = "Reporter"

// A Reporter is a value to which errors may be reported.
// It is satisfied by *testing.T.
type Reporter interface {
	Error(args ...interface{})
}

// SetReporter associates an error reporter (such as a testing.T in
// a Go test) with the Starlark thread so that Starlark programs may
// report errors to it.
func SetReporter(thread *starlark.Thread, r Reporter) {
	thread.SetLocal(localKey, r)
}

// GetReporter returns the Starlark thread's error reporter.
// It must be preceded by a call to SetReporter.
func GetReporter(thread *starlark.Thread) Reporter {
	r, ok := thread.Local(localKey).(Reporter)
	if !ok {
		panic("internal error: starlarktest.SetReporter was not called")
	}
	return r
}

var (
	//go:embed assert.star
	assertSrc []byte

	once      sync.Once
	assert    starlark.StringDict
	assertErr error
)

// LoadAssertModule loads the assert module.
// It is concurrency-safe and idempotent.
func LoadAssertModule(thread *starlark.Thread) (starlark.StringDict, error) {
	once.Do(func() {
		predeclared := starlark.StringDict{
			"error":   starlark.NewBuiltin("error", error_),
			"catch":   starlark.NewBuiltin("catch", catch),
			"matches": starlark.NewBuiltin("matches", matches),
			"module":  starlark.NewBuiltin("module", starlarkstruct.MakeModule),
			"_freeze": starlark.NewBuiltin("freeze", freeze),
		}
		// TODO: remerge with starlarktest.
		filename := "starlarkassert/assert.star"
		assert, assertErr = starlark.ExecFile(thread, filename, assertSrc, predeclared)
	})
	return assert, assertErr
}

// catch(f) evaluates f() and returns its evaluation error message
// if it failed or None if it succeeded.
func catch(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var fn starlark.Callable
	if err := starlark.UnpackArgs("catch", args, kwargs, "fn", &fn); err != nil {
		return nil, err
	}
	if _, err := starlark.Call(thread, fn, nil, nil); err != nil {
		return starlark.String(err.Error()), nil
	}
	return starlark.None, nil
}

// matches(pattern, str) reports whether string str matches the regular expression pattern.
func matches(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var pattern, str string
	if err := starlark.UnpackArgs("matches", args, kwargs, "pattern", &pattern, "str", &str); err != nil {
		return nil, err
	}
	ok, err := regexp.MatchString(pattern, str)
	if err != nil {
		return nil, fmt.Errorf("matches: %s", err)
	}
	return starlark.Bool(ok), nil
}

// error(x) reports an error to the Go test framework.
func error_(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, _ []starlark.Tuple) (starlark.Value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("error: got %d arguments, want 1", len(args))
	}
	buf := new(strings.Builder)
	stk := thread.CallStack()
	stk.Pop()
	fmt.Fprintf(buf, "%sError: ", stk)
	if s, ok := starlark.AsString(args[0]); ok {
		buf.WriteString(s)
	} else {
		buf.WriteString(args[0].String())
	}
	GetReporter(thread).Error(buf.String())
	return starlark.None, nil
}

// freeze(x) freezes its operand.
func freeze(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if len(kwargs) > 0 {
		return nil, fmt.Errorf("freeze does not accept keyword arguments")
	}
	if len(args) != 1 {
		return nil, fmt.Errorf("freeze got %d arguments, wants 1", len(args))
	}
	args[0].Freeze()
	return args[0], nil
}

type Runner func(thread *starlark.Thread, test func())

func (r Runner) run(thread *starlark.Thread, test func()) {
	if r != nil {
		r(thread, test)
		return
	}
	test()
}

// Testing is passed to starlark functions.
type Testing struct {
	t      *testing.T
	frozen bool
}

func (t *Testing) String() string        { return "<testing>" }
func (t *Testing) Type() string          { return "testing.t" }
func (t *Testing) Freeze()               { t.frozen = true }
func (t *Testing) Truth() starlark.Bool  { return t.t != nil }
func (t *Testing) Hash() (uint32, error) { return 0, fmt.Errorf("unhashable type: %s", t.Type()) }

var testingMethods = map[string]*starlark.Builtin{
	"run": starlark.NewBuiltin("testing.t.run", testingRun),
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
		"blob.bucket.write_all", args, kwargs, "name", &name, "fn", &fn,
	); err != nil {
		return nil, err
	}

	var (
		val starlark.Value
		err error
	)

	t.t.Run(name, func(t *testing.T) {
		tval := &Testing{t: t}
		val, err = starlark.Call(thread, fn, starlark.Tuple{tval}, nil)
	})
	return val, err
}

// RunTests runs starlark files as a test suite.
// Each function with the prefix "test_" is called in parallel as a t.Run func.
func RunTests(t *testing.T, pattern string, globals starlark.StringDict, runner Runner) {
	files, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatal(err)
	}

	errorf := func(t *testing.T, filename string, err error) {
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

	newThread := func(t *testing.T, name string) *starlark.Thread {
		t.Helper()

		thread := &starlark.Thread{
			Name:  name,
			Print: func(_ *starlark.Thread, msg string) { t.Log(msg) },
			Load: func(thread *starlark.Thread, module string) (starlark.StringDict, error) {
				if module == "assert.star" {
					return LoadAssertModule(thread)
				}
				return nil, fmt.Errorf("unknown module %s", module)
			},
		}
		SetReporter(thread, t)
		return thread
	}

	for _, filename := range files {
		src, err := ioutil.ReadFile(filename)
		if err != nil {
			t.Fatal(err)
		}

		thread := newThread(t, filename)
		test := func() {
			values, err := starlark.ExecFile(thread, filename, src, globals)
			if err != nil {
				errorf(t, filename, err)
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

					tt := &Testing{t: t}
					thread := newThread(t, filename+"/"+key)
					test := func() {
						if _, err := starlark.Call(thread, val, starlark.Tuple{tt}, nil); err != nil {
							errorf(t, filename, err)
						}
					}
					runner.run(thread, test)
				})
			}

		}
		runner.run(thread, test)
	}
}
