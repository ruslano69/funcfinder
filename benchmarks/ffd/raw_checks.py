"""Check raw CLI contracts and existing raw-cleaner profiles against Go."""
import argparse
import pathlib
import subprocess
import tempfile


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--ffd", required=True)
    ap.add_argument("--go", required=True)
    args = ap.parse_args()
    go, d = [str(pathlib.Path(p).resolve()) for p in (args.go, args.ffd)]
    checks = 0

    def same(flags, code=0):
        nonlocal checks
        a, b = [subprocess.run([exe, *flags], capture_output=True) for exe in (go, d)]
        assert a.returncode == b.returncode == code and a.stdout == b.stdout, (flags,a.returncode,b.returncode,a.stdout,b.stdout,a.stderr,b.stderr)
        checks += 1

    fixtures = {
        "go": ("go", "package p\nfunc f() {\n}\n"),
        "c": ("c", "int f(void) {\n}\n"),
        "cpp": ("cpp", "int f() {\n}\n"),
        "cs": ("cs", "class C {\n void f() {\n }\n}\n"),
        "java": ("java", "class C {\n void f() {\n }\n}\n"),
        "d": ("d", "void f() {\n}\n"),
        "js": ("js", "function f() {\n}\n"),
        "ts": ("ts", "function f(): void {\n}\n"),
        "py": ("py", "def f():\n pass"),
        "ruby": ("rb", "def f\nend\n"),
        "rust": ("rs", "fn f() {\n}\n"),
        "kotlin": ("kt", "fun f() {\n}\n"),
        "swift": ("swift", "func f() {\n}\n"),
        "scala": ("scala", "def f(): Unit = {\n}\n"),
        "php": ("php", "<?php\nfunction f() {\n}\n"),
    }
    build = pathlib.Path(__file__).resolve().parents[2] / "build"
    with tempfile.TemporaryDirectory(prefix=".ffd-raw-", dir=build) as folder:
        root = pathlib.Path(folder)
        assert root.resolve().parent == build.resolve()
        for lang, (ext, source) in fixtures.items():
            directory = root / lang
            directory.mkdir()
            path = directory / ("raw с пробелами." + ext)
            path.write_bytes(source.replace("\n", "\r\n").encode())
            base = ["--inp", str(path), "--source", lang]
            for raw in ("--raw", "--raw=false"):
                for flags in (["--map"], ["--map", "--json"], ["--map", "--extract"], ["--tree"], ["--tree-full"], ["--func", "f"]):
                    same([*base, raw, *flags])
                same(["--dir", str(directory), raw, "--json", "--workers", "1"])
            same([*base, "--names", "--raw=false"], 1)
            same([*base, "--doc", "f", "--raw=false"], 1)
            same([*base, "--map", "--raw=bad"], 2)
        significant = {
            "go": "package p\nfunc f() {\n s := `\n }\n `\n _ = s\n}\nfunc g() {}\n",
            "d": "void f() {\n auto s = `\n }\n `;\n}\nvoid g() {}\n",
            "js": "function f() {\n const s = `\n }\n `;\n}\nfunction g() {}\n",
            "ts": "function f() {\n const s = `\n }\n `;\n}\nfunction g() {}\n",
            "cs": 'class C {\n void f() {\n var s = @"one\\";\n }\n void g() {}\n}\n',
        }
        for lang, source in significant.items():
            path = root / ("significant." + lang)
            path.write_text(source, encoding="utf-8")
            base = ["--inp", str(path), "--source", lang]
            for raw in ([], ["--raw"], ["--raw=false"]):
                code = 2 if lang == "cs" and raw == ["--raw"] else 0
                same([*base, "--map", "--json", *raw], code)
                same([*base, "--map", "--extract", *raw], code)
                if lang == "go":
                    same([*base,"--all","--map","--json",*raw])
                    same([*base,"--tree-full",*raw])
            if lang == "go":
                same([*base,"--map","--json","--raw","--lines","2:8"])
            normal, raw = [subprocess.run([d, *base, "--map", "--json", *flags], capture_output=True) for flags in ([], ["--raw"])]
            assert (normal.stdout != raw.stdout) == (lang != "d"), (lang, normal.stdout, raw.stdout)
            checks += 1
    print(f"{checks} raw flag/output checks passed")


if __name__ == "__main__":
    main()
