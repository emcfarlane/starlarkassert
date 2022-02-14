package starlarkassert

import (
	"fmt"
	"io"
	"regexp"
	"runtime/pprof"
)

// TestDeps is an implementation of the testing.testDeps interface,
// suitable for passing to testing.MainStart.
type TestDeps struct {
	importPath string
}

var matchPat string
var matchRe *regexp.Regexp

func (TestDeps) MatchString(pat, str string) (result bool, err error) {
	if matchRe == nil || matchPat != pat {
		matchPat = pat
		matchRe, err = regexp.Compile(matchPat)
		if err != nil {
			return
		}
	}
	return matchRe.MatchString(str), nil
}

func (TestDeps) StartCPUProfile(w io.Writer) error {
	return pprof.StartCPUProfile(w)
}

func (TestDeps) StopCPUProfile() {
	pprof.StopCPUProfile()
}

func (TestDeps) WriteProfileTo(name string, w io.Writer, debug int) error {
	return pprof.Lookup(name).WriteTo(w, debug)
}

func (t TestDeps) ImportPath() string {
	return t.importPath
}

func (t *TestDeps) StartTestLog(w io.Writer) {}

func (TestDeps) StopTestLog() error { return nil }

// SetPanicOnExit0 tells the os package whether to panic on os.Exit(0).
func (TestDeps) SetPanicOnExit0(v bool) {
	// TODO?
	fmt.Println("SetPanicOnExit", v)
}
