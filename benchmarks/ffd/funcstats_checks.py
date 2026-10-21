"""Check the D funcstats CLI against Go and explicit call/metric/use truth."""
import argparse
import ast
import json
import pathlib
import os
import subprocess
import tempfile


SAMPLES = {
    "c": ("c", "void run() {\n ping();\n ping();\n pong();\n}\n"),
    "cpp": ("cpp", "void run() {\n ping();\n ping();\n pong();\n}\n"),
    "cs": ("cs", "class C {\n void run() {\n ping();\n ping();\n pong();\n }\n}\n"),
    "d": ("d", "void run() {\n ping();\n ping();\n pong();\n}\n"),
    "go": ("go", "package p\nfunc run() {\n ping()\n ping()\n pong()\n}\n"),
    "java": ("java", "class C {\n void run() {\n ping();\n ping();\n pong();\n }\n}\n"),
    "js": ("js", "function run() {\n ping();\n ping();\n pong();\n}\n"),
    "ts": ("ts", "function run(): void {\n ping();\n ping();\n pong();\n}\n"),
    "kotlin": ("kt", "fun run() {\n ping()\n ping()\n pong()\n}\n"),
    "php": ("php", "<?php\nfunction run() {\n ping();\n ping();\n pong();\n}\n"),
    "py": ("py", "def run():\n ping()\n ping()\n pong()\n"),
    "ruby": ("rb", "def run\n ping()\n ping()\n pong()\nend\n"),
    "rust": ("rs", "fn run() {\n ping();\n ping();\n pong();\n}\n"),
    "scala": ("scala", "def run(): Unit = {\n ping()\n ping()\n pong()\n}\n"),
    "swift": ("swift", "func run() {\n ping()\n ping()\n pong()\n}\n"),
}

IMPORTS = {
    "c": '#include <stdio.h>\n', "cpp": '#include <vector>\n', "cs": 'using System.Text;\n',
    "d": 'import std.stdio;\n', "go": 'import "fmt"\n', "java": 'import java.util.List;\n',
    "js": 'import {x} from "pkg";\n', "ts": 'import {x} from "pkg";\n', "kotlin": 'import java.util.List\n',
    "php": 'use Foo\\Bar;\n', "py": 'from pathlib import Path\n', "ruby": 'require "pkg"\n',
    "rust": 'use std::fmt;\n', "scala": 'import scala.util.Try\n', "swift": 'import Foundation\n',
}


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--d", required=True)
    ap.add_argument("--go", required=True)
    a = ap.parse_args()
    d, go = [str(pathlib.Path(p).resolve()) for p in (a.d, a.go)]
    checks = 0

    def run(exe, args, code=0):
        p = subprocess.run([exe, *map(str, args)], capture_output=True)
        assert p.returncode == code, (exe, args, p)
        return p

    def compare(args, json_output=True):
        nonlocal checks
        before, after = [run(exe, args) for exe in (go, d)]
        b, c = (json.loads(before.stdout), json.loads(after.stdout)) if json_output else (before.stdout, after.stdout)
        assert b == c, (args, b, c)
        assert before.stderr == after.stderr == b"", (args, before.stderr, after.stderr)
        checks += 1
        return c

    for flag in ("--help", "-h"):
        compare([flag], False)
    assert b"funcstats" in run(d, ["--version"]).stdout
    checks += 1
    work = pathlib.Path(__file__).resolve().parents[2] / "build"
    work.mkdir(exist_ok=True)
    with tempfile.TemporaryDirectory(prefix="funcstats-checks-", dir=work) as tmp:
        root = pathlib.Path(tmp)
        oracle_source = root/"oracle.go"
        oracle_source.write_bytes(pathlib.Path(__file__).with_name("funcstats_go_ast.go.txt").read_bytes())
        oracle = root/"oracle.exe"
        env = dict(os.environ, GOCACHE=str(work/"funcstats-ast-cache"))
        built = subprocess.run(["go","build","-o",str(oracle),str(oracle_source)],capture_output=True,env=env)
        assert built.returncode == 0, built.stderr
        for lang, (ext, src) in SAMPLES.items():
            file = root / ("fixture." + ext)
            file.write_text(src, encoding="utf-8", newline="")
            obj = compare([file, "-l", lang, "-j"])
            counts = {c["name"]: c["count"] for c in obj["top_calls"]}
            assert counts == {"ping": 2, "pong": 1}, (lang, counts)
            assert obj["total_lines"] == obj["code_lines"] == len(src.splitlines()), (lang, obj)
            assert obj["blank_lines"] == obj["comment_lines"] == 0
            assert obj["file_size_kb"] == len(src.encode()) / 1024
            checks += 1
            compare([file, "-l", lang], False)
            compare([file, "--json", "-n", "1"])
            compare(["-n", "-1", file, "-j"])
            # No phantom EOF line, and empty JSON arrays rather than null.
            file.write_bytes(b"")
            compare([file, "-j"])
            file.write_bytes(b"\n\r\n")
            compare([file, "-j"])
            imported = src.replace("package p\n", "package p\n"+IMPORTS[lang]) if lang == "go" else src.replace("<?php\n", "<?php\n"+IMPORTS[lang]) if lang == "php" else IMPORTS[lang]+src
            file.write_text(imported, encoding="utf-8")
            obj = compare([file, "-j"])
            assert len(obj["imports"]) == 1, (lang, obj)
            checks += 1
        # Independent Python AST calls, including recursion on the definition line,
        # split signatures, comments, decorators and misleading literal text.
        py = root / "truth.py"
        src = "@decorate\ndef unused():\n pass\n\ndef recur(n): return recur(n-1) if n else 0\ndef caller(\n):\n text = 'ghost()'\n # ghost()\n ping()\n callback\n ()\n"
        py.write_text(src, encoding="utf-8")
        obj = compare([py, "-j"])
        expected = {}
        for node in ast.walk(ast.parse(src)):
            if isinstance(node, ast.Call) and isinstance(node.func, ast.Name):
                expected[node.func.id] = expected.get(node.func.id, 0) + 1
        assert {r["name"]: r["count"] for r in obj["top_calls"]} == expected
        checks += 1
        for ext, lang, src, truth in [
            ("java", "java", "class C {\n @Deprecated\n public int\n unused() {return 0;}\n int\n recur(int n) {return recur(n-1);}\n void inline() {inline();}\n}\n", {"recur":1,"inline":1}),
            ("cs", "cs", "class C {\n C() {}\n int Recur(int n) {return Recur(n-1);}\n void Inline() => Inline();\n ~C() {}\n public static implicit operator C(int n) => new C();\n}\n", {"Recur":1,"Inline":1,"C":1}),
            ("cjs", "js", "function $cash(n) {return $cash(n-1);}\nconst arrow = n => arrow(n);\nconst alias = function inner(n) {return inner(n-1);};\nfunction\nsplit() {}\n", {"$cash":1,"arrow":1,"inner":1}),
        ]:
            file = root / ("recursion."+ext); file.write_text(src, encoding="utf-8")
            obj = compare([file, "-l", lang, "-j"])
            assert {r["name"]:r["count"] for r in obj["top_calls"]} == truth, obj
            checks += 1
        tree = root / "tree"; tree.mkdir()
        (tree / "a").mkdir(); (tree / "a" / "z.go").write_text("package p\nfunc nested() {ping()}\n")
        (tree / "a.go").write_text("package p\nfunc run() {ping(); pong()}\n")
        (tree / "b.go").write_text("package p\nfunc second() {ping()}\n")
        (tree / "other.js").write_text("function other() {wrong();}\n")
        (tree / ".hidden.go").write_text("package p\nfunc hidden() {}\n")
        (tree / "ignored.go").write_text("package p\nfunc ignored() {}\n")
        (tree / ".gitignore").write_text("ignored.go\n")
        for extra in ([], ["-n","1"], ["-n","-1"], ["--exclude","b.go"], ["--exclude","a/*.go","--exclude","b.go"], ["--exclude","*.go"], ["--exclude","[ab].go"]):
            compare(["--dir",tree,"-j",*extra])
            compare(["--dir",tree,*extra], False)
        compare(["--dir",tree,"-l","js","-j"])
        # .h belongs to both C and C++; forced C still includes it.
        (tree / "header.h").write_text("void run() {ping();}\n")
        compare(["--dir",tree,"-l","c","-j"])
        uses = root / "uses"; uses.mkdir(); (uses / "internal").mkdir()
        (uses / "a.go").write_text("package p\nfunc Used() {}\nfunc dead() {}\nfunc Recursive() {Recursive()}\nfunc Exported() {}\nfunc init() {}\nfunc Callback() {}\nvar keep = Callback\ntype T struct{}\nfunc (T) String() string {return \"\"}\n")
        (uses / "a_test.go").write_text("package p\nfunc TestUsed() {Used()}\n")
        (uses / "internal" / "b.go").write_text("package internal\nfunc Public() {}\n")
        obj = compare(["--dir",uses,"--uncalled","-j"])
        assert obj["considered"] == 6, obj
        assert {(c["name"],c["exported"]) for c in obj["candidates"]} == {("dead",False),("Exported",True),("Public",False)}, obj
        checks += 1
        compare(["--dir",uses,"--uncalled"], False)
        compare(["--dir",uses,"--uncalled","--exclude","*_test.go","-j"])
        decorated = root / "decorated"; decorated.mkdir()
        (decorated/"sample.py").write_text("@decorate\ndef unused(): pass\ndef recur(n): return recur(n)\ndef via(): pass\nkeep = via\ndef main(): pass\n")
        obj = json.loads(run(d,["--dir",decorated,"--uncalled","-j"]).stdout)
        assert obj["considered"] == 3 and [c["name"] for c in obj["candidates"]] == ["unused"], obj
        checks += 1
        # Sequential reads must not corrupt names retained from thread-local arenas.
        first = run(d,["--dir",uses,"-j"]).stdout
        for _ in range(3):
            assert run(d,["--dir",uses,"-j"]).stdout == first
            checks += 1
        for content in (b"package p\r\n// comment\r\n\r\nfunc f() {ping()}\r\n", b"package p\n//"+b"x"*131072+b"\nfunc f(){ping()}\n", "package p\nfunc f(){пинг()}\n".encode(), b"package p\n//\xff\xfe\nfunc f(){ping()}\n"):
            file = root/"edge.go"; file.write_bytes(content)
            compare([file,"-j"])
        # Go AST separates interface signatures, control syntax and composite type conversions
        # from calls, and includes generic/indexed invocations missed by a name+'(' regex.
        file = root/"ast_truth.go"
        file.write_text("package p\ntype I interface {Call([]byte)}\nfunc Generic[T any](x T) {}\nfunc run(values []func(), objects []I) {\n _ = []byte(nil)\n _ = map[string]int(nil)\n _ = chan int(nil)\n if (true) {Generic[int](1)}\n values[0]()\n objects[0].Call(nil)\n _ = []int{0}[0]\n ping()\n}\n")
        config = pathlib.Path(__file__).with_name("ffd_config.json")
        truth = json.loads(run(str(oracle),[config,file]).stdout)[str(file)]
        obj = json.loads(run(d,[file,"-j"]).stdout)
        assert {r["name"]:r["count"] for r in obj["top_calls"]} == truth, (truth,obj)
        checks += 1
        # Source truth takes precedence over Go's raw-line import matches inside comments/literals.
        for ext, src in [
            ("go", 'package p\n/*\nimport "fake"\n*/\nimport (\n "real"\n)\nfunc f(){ping()}\n'),
            ("cpp", '/*\n#include "fake"\n*/\n#include "real"\nvoid f(){ping();}\n'),
            ("py", 'text = """\nimport fake\n@fake\n"""\nimport real\ndef f():\n ping()\n'),
        ]:
            file = root/("metadata."+ext); file.write_text(src, encoding="utf-8")
            obj = json.loads(run(d,[file,"-j"]).stdout)
            assert obj["imports"] == ["real"] and obj["decorators"] == [], obj
            assert {r["name"]:r["count"] for r in obj["top_calls"]} == {"ping":1}, obj
            assert obj["total_lines"] == obj["code_lines"] + obj["comment_lines"] + obj["blank_lines"]
            checks += 1
        private_dir = root / "private-js"
        private_dir.mkdir()
        private_file = private_dir / "private.js"
        private_file.write_text("class C { #ping(){} run(){this.#ping(); external?.ping()} }\n", encoding="utf-8")
        obj = json.loads(run(d, [private_file, "-j"]).stdout)
        assert {r["name"]: r["count"] for r in obj["top_calls"]} == {"#ping": 1, "ping": 1}, obj
        uses = json.loads(run(d, ["--dir", private_dir, "--uncalled", "-j"]).stdout)
        assert "#ping" not in {c["name"] for c in uses["candidates"]}, uses
        checks += 1
        # Pooled token/call scratch must not corrupt retained count-map keys across files/workers.
        pooled = root / "pooled"
        pooled.mkdir()
        for i in range(40):
            repeats = 2049 if i == 0 else 257
            (pooled / f"f{i:02d}.go").write_text(
                f"package p\nfunc keep{i}() {{}}\nfunc dead{i}() {{" + f"keep{i}();" * repeats + "}\n")
        obj = json.loads(run(d, ["--dir", pooled, "-j"]).stdout)
        assert obj["total_files"] == 40
        for row in obj["files"]:
            i = int(pathlib.Path(row["file"]).stem[1:])
            assert {r["name"]: r["count"] for r in row["top_calls"]} == {f"keep{i}": 2049 if i == 0 else 257}, row
        checks += 1
        unused = json.loads(run(d, ["--dir", pooled, "--uncalled", "-j"]).stdout)
        assert {r["name"] for r in unused["candidates"]} == {f"dead{i}" for i in range(40)}, unused
        checks += 1
        for args in ([], ["--uncalled"], ["missing.go"], [py,"-l","bad"], ["--dir",tree,"--exclude","["], ["--dir",tree,"--exclude","[]"], ["--dir",tree,"--exclude","[a-]"]):
            for exe in (go,d): run(exe,args,1)
            checks += 1
        # D rejects malformed options instead of Go's silent ignore/coercion.
        for args in ([py,"--bogus"], [py,"-n","junk"], ["--dir"]):
            run(d,args,1); checks += 1
    print(f"{checks} funcstats CLI/truth checks passed across 15 languages")


if __name__ == "__main__":
    main()
