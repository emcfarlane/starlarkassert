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

## test

### test·error

`t.error(msg)` reports the error msg to the test runner.

| Parameter | Type | Description |
| --------- | ---- | ----------- |
| msg | any | Message. |

### test·fail

`t.fail()` fails the test and halts test running.

### test·fatal

`t.fatal(msg)` reports the error msg to the test runner, and fails the test.

| Parameter | Type | Description |
| --------- | ---- | ----------- |
| msg | any | Message. |

### test·freeze

`t.freeze(val)` freze .

| Parameter | Type | Description |
| --------- | ---- | ----------- |
| val | any | Value to freeze. |

### test·run

`t.run(subtest)` run .

| Parameter | Type | Description |
| --------- | ---- | ----------- |
| subtest | func | Function to run as a subtest. |

### test·skip

`t.skip()` skips the current test.

### test·equal

`t.equal(a, b)` compares two values of the same type are equal.

| Parameter | Type | Description |
| --------- | ---- | ----------- |
| a | T | Value expected. |
| b | T | Value given. |

### test·not_equal

`t.not_equal(a, b)` compares two values of the same type are not equal, r

| Parameter | Type | Description |
| --------- | ---- | ----------- |
| a | Value | Value expected. |
| b | Value | Value given. |

### test·less_than

`t.less_than(a, b)` compares two values of the same type are less than.

| Parameter | Type | Description |
| --------- | ---- | ----------- |
| a | Value | Value expected. |
| b | Value | Value given. |

### test·true

`t.true(a, msg)` checks truthyness reporting the message if falsy.

| Parameter | Type | Description |
| --------- | ---- | ----------- |
| a | Value | Value expected to be truthy. |
| msg | String | Message to report on falsyness. |

### test·contains

`t.contains(a, b)` checks `b` is in `a`.

| Parameter | Type | Description |
| --------- | ---- | ----------- |
| a | Iterable | Iterable item. |
| b | Value | Value expected. |

| msg | any | Message to report on error. |

### test·fails

`t.fails(f, pattern)` runs the function and checks the returned error matches the regex pattern.

| Parameter | Type | Description |
| --------- | ---- | ----------- |
| f | func | Value to run. |
| pattern | string | Regex pattern to match. |


## bench

Bench is a superset of test. All attributes are included plus the following.

### bench·restart

`b.restart()` the benchmark clock.

### bench·start

`b.start()` the benchmark clock.

### bench·stop

`b.stop()` the benchmark clock.

### bench·n

`b.n` returns the current benchmark iteration size.
