"""Compare tree formatting and flag contracts with Go on controlled declarations."""
import argparse
import pathlib
import subprocess
import tempfile


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--ffd", required=True)
    ap.add_argument("--go", required=True)
    args = ap.parse_args()
    executables = [str(pathlib.Path(p).resolve()) for p in (args.go, args.ffd)]
    count = 0

    def same(flags, code=0):
        nonlocal count
        a, b = [subprocess.run([exe, *flags], capture_output=True) for exe in executables]
        assert a.returncode == b.returncode == code and a.stdout == b.stdout, (flags, a.returncode, b.returncode, a.stdout, b.stdout, a.stderr, b.stderr)
        count += 1

    sources = {
        "go": ("go", "package p\nfunc f() {\n println(1)\n}\ntype C struct{}"),
        "cpp": ("cpp", "int f() {\n return 1;\n}\nclass C {};"),
        "c": ("c", "int f(void) {\n return 1;\n}"),
        "cs": ("cs", "class C {}"),
        "java": ("java", "class C {}"),
        "js": ("js", "function f() {\n return 1;\n}\nclass C {}"),
        "ts": ("ts", "function f(): number {\n return 1;\n}\nclass C {}"),
        "d": ("d", "int f() {\n return 1;\n}\nclass C {}"),
        "py": ("py", "def f():\n return 1\nclass C:\n pass"),
        "ruby": ("rb", "def f\n 1\nend"),
        "rust": ("rs", "fn f() {\n}\nstruct C {}"),
        "kotlin": ("kt", "fun f(): Int {\n return 1\n}\nclass C {}"),
        "swift": ("swift", "func f() -> Int {\n return 1\n}\nstruct C {}"),
        "scala": ("scala", "def f(): Int = {\n 1\n}\nclass C {}"),
        "php": ("php", "<?php\nfunction f() {\n return 1;\n}\nclass C {}"),
    }
    build = pathlib.Path(__file__).resolve().parents[2] / "build"
    with tempfile.TemporaryDirectory(prefix=".ffd-tree-", dir=build) as folder:
        root = pathlib.Path(folder)
        assert root.resolve().parent == build.resolve()
        for lang, (ext, source) in sources.items():
            directory = root / lang
            directory.mkdir()
            path = directory / ("tree with spaces." + ext)
            path.write_bytes(source.replace("\n", "\r\n").encode())
            base = ["--inp", str(path), "--source", lang]
            for tree in ("--tree", "--tree-full"):
                same([*base, tree], 2 if lang in ("cs", "java") else 0)
                for mode in (["--all"], ["--struct"]):
                    same([*base, *mode, tree], 2 if lang in ("c", "ruby") and mode == ["--struct"] else 0)
                same([*base, tree, "--json"], 2 if lang in ("cs", "java") else 0)
                same([*base, tree, "--extract"], 2 if lang in ("cs", "java") else 0)
                for workers in (1, 8):
                    for mode in ([], ["--all"], ["--struct"]):
                        same(["--dir", str(directory), tree, *mode, "--workers", str(workers)])
                same([*base, tree, "--func", "f"], 1)
                same([*base, tree, "--struct", "--type", "C"], 1)
            same([*base, "--tree", "--tree-full"], 1)
            same([*base, "--names", "--tree=false"], 1)
        # Exercise repeated declarations, nested functions and multi-level paths.
        extra = root / "nested" / "sub"
        extra.mkdir(parents=True)
        path = extra / "nested.js"
        path.write_text("function outer() {\n function inner() {\n  function deep() {\n  }\n }\n}\nclass C {\n f() {\n }\n}\n", encoding="utf-8")
        for tree in ("--tree", "--tree-full"):
            same(["--inp", str(path), "--source", "js", tree])
            same(["--inp", str(path), "--source", "js", "--all", tree])
            same(["--dir", str(root / "nested"), tree])
            same(["--dir", str((root / "nested").relative_to(pathlib.Path.cwd())), tree])
        fixtures = [
            ("go", "go", "package p\ntype C struct {\n X int\n Y string\n}\nfunc (c C) f() {\n}\nfunc g() {\n}\n"),
            ("cs", "cs", "class C {\n int f() {\n return 1;\n }\n}\n"),
            ("java", "java", "class C {\n int f() {\n return 1;\n }\n}\n"),
            ("js", "js", "function outer() {\n function a() {\n }\n function b() {\n }\n}\n"),
            ("cpp", "cpp", "int f(\n int x\n) {\n return x;\n}\nint f() {\n return 0;\n}\n"),
            ("py", "py", "@decorate\ndef f(\n x\n):\n return x"),
        ]
        for lang, ext, source in fixtures:
            path = root / ("extra." + ext)
            path.write_text(source, encoding="utf-8")
            base = ["--inp", str(path), "--source", lang]
            for tree in ("--tree", "--tree-full"):
                same([*base, tree])
                if lang == "go":
                    same([*base, "--all", tree])
                    same([*base, "--struct", tree])
                    same([*base, tree, "--lines", "2:9"])
                for flags in (["--json"], ["--extract"], ["--map"]):
                    same([*base, tree, *flags])
    print(f"{count} tree CLI comparisons passed")


if __name__ == "__main__":
    main()
