# starlarkassert

[![Go Reference](https://pkg.go.dev/badge/github.com/emcfarlane/starlarkassert.svg)](https://pkg.go.dev/github.com/emcfarlane/starlarkassert)

Package starlarkassert binds starlark scripts to go's testing framework.

```python
# tests are prefixed with "test_"
def test_are_prefix(t):
    t.true(True)
    print("here")

# tests can run subtests with "t.run()"
def test_subtest(t):
    for name in ["test", "names"]:
        t.run(name, lambda t: print(name))
```

```python
# benches are prefixed with "bench_"
def bench_append(b):
    a = []
    b.restart()
    for i in range(b.n):
        a.append(i)
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
