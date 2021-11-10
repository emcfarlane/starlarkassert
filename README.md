# starlarkassert

Fork of the original, embeds assert function and includes integrations for go's testing.T.

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
