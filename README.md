# starlarkassert

[![Go Reference](https://pkg.go.dev/badge/github.com/emcfarlane/starlarkassert.svg)](https://pkg.go.dev/github.com/emcfarlane/starlarkassert)

Superset of the original starlarktest package, includes integrations for go's testing.T.

```python
load("assert.star", "assert")

assert.true(True)

# test are prefixed with "test_"
def test_are_prefix(t):
    print("here")

# tests can run subtests with "t.run()"
def test_subtest(t):
    for name in ["test", "names"]:
        t.run(name, lambda t: print(name))

```

Integrate starlark Scripts with go's test framework:
```go
func TestScript(t *testing.T) {
	globals := starlark.StringDict{
		"struct": starlark.NewBuiltin("struct", starlarkstruct.Make),
	}
	RunTests(t, "testdata/*.star", globals)
}
```
