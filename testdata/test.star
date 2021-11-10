# Tests of Starlark 'assert' extension.

load("assert.star", "assert")

assert.true(True)

def test_here(t):
    print("here")

def test_array(t):
    for name in ["lord", "of", "the", "rings"]:
        print("name:", name)

def test_t_run(t):
    for name in ["harry", "potter"]:
        t.run(name, lambda t: print(name))
