package starlarkassert

import (
	"errors"
	"io"
	"reflect"
	"time"
)

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
