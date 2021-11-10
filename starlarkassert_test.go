package starlarkassert

import (
	"testing"
)

func TestRunTests(t *testing.T) {
	RunTests(t, "testdata/*.star", nil, nil)
}
