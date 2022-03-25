package starlarkassert

import "testing"

func _check() {
	var deps MatchStringOnly = nil
	testing.MainStart(deps, nil, nil, nil, nil)
}
