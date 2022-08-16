package starlarkassert

import (
	_ "embed"
	"fmt"
	"regexp"
	"testing"

	. "go.starlark.net/starlark"
	"go.starlark.net/syntax"
)

type method struct {
	recv Value
	name string
	fn   func(*Thread, Tuple, []Tuple) (Value, error)
}

func (m method) Name() string          { return m.name }
func (m method) Freeze()               {}
func (m method) Hash() (uint32, error) { return 0, nil }
func (m method) String() string {
	return fmt.Sprintf("<builtin_method %s of %s value>", m.Name(), m.recv.Type())
}
func (m method) Type() string { return "builtin_method" }
func (m method) Truth() Bool  { return true }
func (m method) CallInternal(thread *Thread, args Tuple, kwargs []Tuple) (Value, error) {
	return m.fn(thread, args, kwargs)
}

type tmethod struct {
	recv Value
	name string
	tb   testing.TB
	fn   func(testing.TB, *Thread, Tuple, []Tuple) (Value, error)
}

func (m tmethod) Name() string          { return m.name }
func (m tmethod) Freeze()               {}
func (m tmethod) Hash() (uint32, error) { return 0, nil }
func (m tmethod) String() string {
	return fmt.Sprintf("<builtin_method %s of %s value>", m.Name(), m.recv.Type())
}
func (m tmethod) Type() string { return "builtin_method" }
func (m tmethod) Truth() Bool  { return true }
func (m tmethod) CallInternal(thread *Thread, args Tuple, kwargs []Tuple) (Value, error) {
	return m.fn(m.tb, thread, args, kwargs)
}

var print_ = Universe["print"].(*Builtin)

func pprint(thread *Thread, args Tuple, kwargs []Tuple) (string, error) {
	var s string

	oldPrint := thread.Print
	thread.Print = func(_ *Thread, msg string) { s = msg }
	defer func() { thread.Print = oldPrint }()

	_, err := print_.CallInternal(thread, args, kwargs)
	return s, err
}

// freeze(x) freezes its operand.
func freeze(_ *Thread, args Tuple, kwargs []Tuple) (Value, error) {
	if len(kwargs) > 0 {
		return nil, fmt.Errorf("freeze does not accept keyword arguments")
	}
	if len(args) != 1 {
		return nil, fmt.Errorf("freeze got %d arguments, wants 1", len(args))
	}
	args[0].Freeze()
	return args[0], nil
}

func terror(t testing.TB, thread *Thread, args Tuple, kwargs []Tuple) (Value, error) {
	s, err := pprint(thread, args, kwargs)
	if err != nil {
		return nil, err
	}
	t.Error(s)
	return True, nil
}

func tskip(t testing.TB, thread *Thread, args Tuple, kwargs []Tuple) (Value, error) {
	s, err := pprint(thread, args, kwargs)
	if err != nil {
		return nil, err
	}
	t.Skip(s)
	return True, nil
}

func tfatal(t testing.TB, thread *Thread, args Tuple, kwargs []Tuple) (Value, error) {
	s, err := pprint(thread, args, kwargs)
	if err != nil {
		return nil, err
	}
	t.Fatal(s)
	return True, nil
}

func tfail(t testing.TB, _ *Thread, args Tuple, kwargs []Tuple) (Value, error) {
	if len(args) > 0 || len(kwargs) > 0 {
		return nil, fmt.Errorf("fail does not accept arguments")
	}
	t.Fail()
	return True, nil
}

func teq(t testing.TB, _ *Thread, args Tuple, kwargs []Tuple) (Value, error) {
	var x, y Value
	if err := UnpackArgs("eq", args, kwargs, "x", &x, "y", &y); err != nil {
		return nil, err
	}
	ok, err := Equal(x, y)
	if err != nil {
		return nil, err
	}
	if !ok {
		if v, diffOk := x.(Diffable); diffOk {
			str, err := v.DiffSameType(y)
			if err != nil {
				return nil, err
			}
			t.Error(str)
		} else {
			t.Errorf(
				"%s != %s", String(x.String()), String(y.String()),
			)
		}
	}
	return Bool(ok), nil
}

func tne(t testing.TB, _ *Thread, args Tuple, kwargs []Tuple) (Value, error) {
	var x, y Value
	if err := UnpackArgs("ne", args, kwargs, "x", &x, "y", &y); err != nil {
		return nil, err
	}
	ok, err := Equal(x, y)
	if err != nil {
		return nil, err
	}
	if ok {
		t.Errorf(
			"%s == %s", String(x.String()), String(y.String()),
		)
	}
	return Bool(!ok), nil
}

func ttrue(t testing.TB, _ *Thread, args Tuple, kwargs []Tuple) (Value, error) {
	var (
		cond Value
		msg  string
	)
	if err := UnpackArgs("true", args, kwargs, "cond", &cond, "msg?", &msg); err != nil {
		return nil, err
	}
	if !bool(cond.Truth()) {
		t.Error(msg)
	}
	return cond.Truth(), nil
}

func tlt(t testing.TB, _ *Thread, args Tuple, kwargs []Tuple) (Value, error) {
	var x, y Value
	if err := UnpackArgs("lt", args, kwargs, "x", &x, "y", &y); err != nil {
		return nil, err
	}
	ok, err := Compare(syntax.LT, x, y)
	if err != nil {
		return nil, err
	}
	if !ok {
		t.Errorf("%s is not less than %s", x, y)
	}
	return Bool(ok), nil
}

func tcontains(t testing.TB, _ *Thread, args Tuple, kwargs []Tuple) (Value, error) {
	var (
		x Iterable
		y Value
	)
	if err := UnpackArgs("contains", args, kwargs, "x", &x, "y", &y); err != nil {
		return nil, err
	}
	iter := x.Iterate()
	defer iter.Done()

	var p Value
	for iter.Next(&p) {
		ok, err := Equal(y, p)
		if err != nil {
			return nil, err
		}
		if ok {
			return True, nil
		}
	}
	t.Errorf("%s does not contain %s", x, y)
	return False, nil
}

func tfails(t testing.TB, thread *Thread, args Tuple, kwargs []Tuple) (Value, error) {
	var (
		f       Callable
		pattern string
	)
	if err := UnpackArgs("fails", args, kwargs, "f", &f, "pattern", &pattern); err != nil {
		return nil, err
	}

	_, err := f.CallInternal(thread, nil, nil)
	if err == nil {
		t.Errorf("evaluation succeeded unexpectedly (want error matching %s)", String(pattern))
		return False, nil
	}
	str := err.Error()
	ok, err := regexp.MatchString(pattern, str)
	if err != nil {
		return nil, fmt.Errorf("matches: %s", err)
	}

	if !ok {
		t.Errorf("regular expression (%s) did not match error (%s)", pattern, str)
	}
	return Bool(ok), nil
}
