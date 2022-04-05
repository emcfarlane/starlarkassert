# Benchmark of Starlark 'assert' extension.

def bench_append(b):
    a = []
    b.restart()
    for i in range(b.n):
        a.append(i)
