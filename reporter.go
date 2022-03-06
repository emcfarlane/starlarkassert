package starlarkassert

import (
	_ "embed"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarktest"
)

// A Reporter is a value to which errors may be reported.
// It is satisfied by *testing.T.
type Reporter = starlarktest.Reporter

// SetReporter associates an error reporter (such as a testing.T in
// a Go test) with the Starlark thread so that Starlark programs may
// report errors to it.
func SetReporter(thread *starlark.Thread, r Reporter) {
	starlarktest.SetReporter(thread, r)
}

// GetReporter returns the Starlark thread's error reporter.
// It must be preceded by a call to SetReporter.
func GetReporter(thread *starlark.Thread) Reporter {
	return starlarktest.GetReporter(thread)
}

// LoadAssertModule loads the assert module.
// It is concurrency-safe and idempotent.
func LoadAssertModule(thread *starlark.Thread) (starlark.StringDict, error) {
	return starlarktest.LoadAssertModule()
}
