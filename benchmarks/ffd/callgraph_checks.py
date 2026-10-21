"""D callgraph: explicit 15-language truth, Go AST, traversal and CLI checks."""
import argparse
import json
import os
import pathlib
import subprocess
import tempfile

SAMPLES = {
    "c": ("c", "void ping() {}\nvoid run() {\n ping(); ping();\n}\n"),
    "cpp": ("cpp", "void ping() {}\nvoid run() {\n ping(); ping();\n}\n"),
    "cs": ("cs", "class C {\n void ping() {}\n void run() {\n ping(); ping();\n }\n}\n"),
    "java": ("java", "class C {\n void ping() {}\n void run() {\n ping(); ping();\n }\n}\n"),
    "d": ("d", "void ping() {}\nvoid run() {\n ping(); ping();\n}\n"),
    "go": ("go", "package p\nfunc ping() {}\nfunc run() {\n ping(); ping()\n}\n"),
    "js": ("js", "function ping() {}\nfunction run() {\n ping(); ping();\n}\n"),
    "ts": ("ts", "function ping(): void {}\nfunction run(): void {\n ping(); ping();\n}\n"),
    "kotlin": ("kt", "fun ping() {}\nfun run() {\n ping(); ping()\n}\n"),
    "swift": ("swift", "func ping() {}\nfunc run() {\n ping(); ping()\n}\n"),
    "rust": ("rs", "fn ping() {}\nfn run() {\n ping(); ping();\n}\n"),
    "php": ("php", "<?php\nfunction ping() {}\nfunction run() {\n ping(); ping();\n}\n"),
    "py": ("py", "def ping():\n pass\ndef run():\n ping(); ping()\n"),
    "ruby": ("rb", "def ping\nend\ndef run\n ping(); ping()\nend\n"),
    "scala": ("scala", "def ping(): Unit = {}\ndef run(): Unit = {\n ping(); ping()\n}\n"),
}


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--d", required=True)
    ap.add_argument("--go", required=True)
    a = ap.parse_args()
    d, go = [str(pathlib.Path(p).resolve()) for p in (a.d, a.go)]
    work = pathlib.Path(__file__).resolve().parents[2] / "build"
    checks = 0

    def run(exe, args, code=0):
        p = subprocess.run([exe, *map(str, args)], capture_output=True)
        assert p.returncode == code, (args, p.returncode, p.stdout, p.stderr)
        return p

    def graph(args):
        nonlocal checks
        checks += 1
        return json.loads(run(d, [*args, "--json"]).stdout)

    def edges(obj):
        return {(e["caller"], e["callee"], e["line"]) for f in obj["files"] or [] for e in f["calls"] or []}

    with tempfile.TemporaryDirectory(prefix="callgraph-checks-", dir=work) as tmp:
        root = pathlib.Path(tmp)
        for lang, (ext, source) in SAMPLES.items():
            f = root / ("sample." + ext)
            f.write_text(source, encoding="utf-8", newline="")
            args = ["--inp", f, "-l", lang]
            obj = graph(args)
            line = next(i for i, s in enumerate(source.splitlines(), 1) if "ping(); ping()" in s)
            assert edges(obj) == {("run", "ping", line)}, (lang, obj)
            assert obj["total_calls"] == 1 and obj["total_functions"] == 2, (lang, obj)
            ref = json.loads(run(go, [*args, "--json"]).stdout)
            assert edges(ref) == edges(obj), (lang, ref, obj)
            rev = graph([*args, "--reverse", "--func", "ping"])
            assert rev == {"function": "ping", "callers": ["run"]}, (lang, rev)

        source = '''package sample
import alias "fmt"
func A() {
 B(); C()
}
func B() { C() }
func C() { D() }
func D() { C(); alias.Println("C()") }
'''
        f = root / "truth.go"
        f.write_text(source, encoding="utf-8", newline="")
        args = ["--inp", f, "-l", "go"]
        obj = graph(args)
        assert edges(obj) == {("A", "B", 4), ("A", "C", 4), ("B", "C", 6), ("C", "D", 7), ("D", "C", 8), ("D", "fmt.Println", 8)}, obj
        assert graph([*args, "-r", "--func", "Println"])["callers"] == ["D"]
        assert graph([*args, "-r", "--func", "fmt.Println"])["callers"] == ["D"]
        assert graph([*args, "-r", "--func", "other.Println"])["callers"] == []
        filtered = graph([*args, "--func", "A", "--depth", 2])
        assert edges(filtered) == {("A", "B", 4), ("A", "C", 4), ("B", "C", 6), ("C", "D", 7)}, filtered
        assert all(row["path"] == str(f) for row in filtered["files"])
        tree = run(d, [*args, "--func", "A"]).stdout.decode()
        assert "C (↑ recursive)" in tree and "C (see above)" in tree, tree

        # Use the compiler's AST independently of either implementation.
        oracle_src = root / "oracle.go"
        oracle_src.write_bytes(pathlib.Path(__file__).with_name("callgraph_go_ast.go.txt").read_bytes())
        oracle = root / "oracle.exe"
        env = dict(os.environ, GOCACHE=str(work / "callgraph-ast-cache"))
        built = subprocess.run(["go", "build", "-o", str(oracle), str(oracle_src)], capture_output=True, env=env)
        assert built.returncode == 0, built.stderr
        ref = subprocess.run([str(oracle)], input=json.dumps([str(f)]).encode(), capture_output=True)
        assert ref.returncode == 0, ref.stderr
        assert edges(json.loads(ref.stdout)) == edges(obj)
        checks += 1

        generic = root / "generic.go"
        generic.write_text('''package sample
// import alias "wrong"
import (
 alias "fmt"
)
func identity[T any](x T) T { return x }
func Привет() { Привет() }
func run() {
 const decoy = `import alias "bogus"; identity()`
 alias.Println(identity[int](1)); Привет()
}
''', encoding="utf-8", newline="\r\n")
        generic_graph = graph(["--inp", generic, "-l", "go"])
        assert edges(generic_graph) == {("Привет", "Привет", 7), ("run", "fmt.Println", 10),
                                        ("run", "identity", 10), ("run", "Привет", 10)}, generic_graph
        ref = subprocess.run([str(oracle)], input=json.dumps([str(generic)]).encode(), capture_output=True)
        assert ref.returncode == 0, ref.stderr
        assert edges(json.loads(ref.stdout)) == edges(generic_graph)
        checks += 1

        # Same-line neighbors and inner named functions do not steal outer calls.
        js = root / "inline.js"
        js.write_text("function ping(){} function pong(){} function outer(){ping();function inner(){pong()}ping()} function next(){pong()}\n", encoding="utf-8")
        inline = graph(["--inp", js, "-l", "js"])
        assert edges(inline) == {("outer", "ping", 1), ("inner", "pong", 1), ("next", "pong", 1)}, inline
        js.write_text("function ping(){} function pong(){} const first=()=>ping(), second=()=>pong();\n", encoding="utf-8")
        inline = graph(["--inp", js, "-l", "js"])
        assert edges(inline) == {("first", "ping", 1), ("second", "pong", 1)}, inline
        js.write_text("class C { #ping(){} run(){this.#ping(); external?.ping()} }\n", encoding="utf-8")
        private = graph(["--inp", js, "-l", "js"])
        assert edges(private) == {("run", "this.#ping", 1), ("run", "external.ping", 1)}, private
        ts = root / "literal_type.ts"
        ts.write_text("function ping(){} function make(): {x:number} {ping(); return {x:1};}\n", encoding="utf-8")
        literal_type = graph(["--inp", ts, "-l", "ts"])
        assert edges(literal_type) == {("make", "ping", 1)}, literal_type
        cpp = root / "initializer.cpp"
        cpp.write_text("void ping(){} struct A{int x,y; A():x{1},y{2}{ping();}};\n", encoding="utf-8")
        initializer = graph(["--inp", cpp, "-l", "cpp"])
        assert edges(initializer) == {("A", "ping", 1)}, initializer
        py = root / "nested.py"
        py.write_text("def ping():\n pass\ndef outer():\n def inner(x=ping()):\n  ping()\n ping()\n", encoding="utf-8")
        nested = graph(["--inp", py, "-l", "py"])
        assert edges(nested) == {("outer", "ping", 4), ("inner", "ping", 5)}, nested

        # Cross-file names are collected before filtering, imports are lexical,
        # and a forced language applies to the entire scan (including .h as C).
        directory = root / "project"
        directory.mkdir()
        (directory / "a.go").write_text("package p\nfunc A() { B() }\n", encoding="utf-8")
        (directory / "b.go").write_text("package p\nfunc B() {}\n", encoding="utf-8")
        (directory / "x.py").write_text("def py():\n py()\n", encoding="utf-8")
        (directory / "ignored.go").write_text("package p\nfunc ignored() { ignored() }\n", encoding="utf-8")
        (directory / ".gitignore").write_text("ignored.go\n", encoding="utf-8")
        obj = graph(["--dir", directory, "-l", "go"])
        assert edges(obj) == {("A", "B", 2)} and obj["total_functions"] == 2, obj
        mixed = graph([directory])
        assert edges(mixed) == {("A", "B", 2), ("py", "py", 2)}, mixed
        assert ("ignored", "ignored", 2) in edges(graph([directory, "--no-gitignore", "-l", "go"]))
        assert graph(["--dir", directory, "--func", "missing"])["files"] is None
        assert graph(["--dir", directory, "--reverse", "--func", "missing"])["callers"] == []
        first = run(d, [directory, "-j"]).stdout
        assert all(run(d, [directory, "-j"]).stdout == first for _ in range(3))
        checks += 1

        # Metadata/source buffers must survive many files on reused workers.
        for i in range(30):
            (directory / f"extra{i}.go").write_text(f"package p\nfunc extra{i}() {{ A() }}\n", encoding="utf-8")
        obj = graph([directory, "-l", "go"])
        assert obj["total_functions"] == 32 and obj["total_calls"] == 31, obj
        header = directory / "only.h"
        header.write_text("void header() { header(); }\n", encoding="utf-8")
        assert edges(graph([directory, "-l", "c"])) == {("header", "header", 1)}
        empty = root / "empty.go"
        empty.write_text("package p\n", encoding="utf-8")
        no_calls = graph(["--inp", empty, "-l", "go"])
        assert no_calls["total_functions"] == no_calls["total_calls"] == 0
        assert no_calls["files"] == [{"path": str(empty), "calls": None}], no_calls

        for bad in ([], ["--inp", f], ["--dir"], ["--depth", "bad"], ["--depth", "-1"],
                    ["--unknown"], ["--dir", directory, "-l", "invalid"],
                    ["--dir", directory, "--inp", f], ["--dir", root / "absent"],
                    ["--inp", root / "absent.go", "-l", "go"]):
            run(d, bad, 1)
            checks += 1
        for flag in ("--help", "-h", "--version"):
            assert b"callgraph" in run(d, [flag]).stdout
            checks += 1
    print(f"callgraph: {checks} checks passed (15 languages, explicit truth and Go AST)")


if __name__ == "__main__":
    main()
