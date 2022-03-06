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

def test_globals(t):
    struct(name = "hello")

a_list = [1, 2, 3]

def test_globals_frozen(t):
    assert.fails(lambda: a_list.append(4), "frozen list")

load("test_load.star", "greet")

def test_load(t):
    assert.eq(greet, "world")
    print("hello,", greet)
