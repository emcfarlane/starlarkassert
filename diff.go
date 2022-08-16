package starlarkassert

import "go.starlark.net/starlark"

// A Diffable is a value that can report it's difference.
type Diffable interface {
	starlark.Comparable

	// DiffSameType compares a value of the same Type().
	// Returns a human-readable report of the difference between the two values.
	// Returns an empty string if the two values are equal.
	// Implementation should be similar to cmp.Diff().
	DiffSameType(y starlark.Value) (string, error)
}
