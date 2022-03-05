# Benchmark of Starlark 'assert' extension.

def bench_method(b):
    a = []
    b.restart()
    for i in range(b.n):
        a.append(i)
