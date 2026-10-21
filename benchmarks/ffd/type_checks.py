"""Compare ffd type/names CLI contracts with Go on controlled fieldless declarations."""
import argparse
import json
import pathlib
import subprocess
import tempfile


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--ffd", required=True)
    ap.add_argument("--go", required=True)
    args = ap.parse_args()
    d, go = (str(pathlib.Path(p).resolve()) for p in (args.ffd, args.go))
    checks = 0

    def same(flags, json_output=False, code=0, c_default=False):
        nonlocal checks
        a, b = [subprocess.run([exe, *flags], capture_output=True) for exe in (go, d)]
        assert a.returncode == b.returncode == code, (flags, a.returncode, b.returncode, a.stderr, b.stderr)
        if not code:
            va, vb = (json.loads(a.stdout), json.loads(b.stdout)) if json_output else (a.stdout, b.stdout)
            if c_default:
                # Existing native C profile includes types even in function directory mode; Go does not.
                assert set(va.splitlines()) <= set(vb.splitlines()), (flags, va, vb)
                extra = set(vb.splitlines()) - set(va.splitlines())
                assert len(extra) == 1 and next(iter(extra)).startswith(b"class\tC\t"), (flags, extra)
            else:
                assert va == vb, (flags, va, vb)
        checks += 1

    build = pathlib.Path(__file__).resolve().parents[2] / "build"
    build.mkdir(exist_ok=True)
    with tempfile.TemporaryDirectory(prefix=".ffd-types-", dir=build) as folder:
        root = pathlib.Path(folder)
        assert root.resolve().parent == build.resolve()
        sources = {
            "go": ("go", "package p\nfunc f() {}\ntype C struct{}"),
            "cpp": ("cpp", "int f() { return 1; }\nclass C {};"),
            "c": ("c", "int f(void) { return 1; }\nstruct C {};"),
            "cs": ("cs", "class C {}"),
            "java": ("java", "class C {}"),
            "js": ("js", "function f() {}\nclass C {}"),
            "ts": ("ts", "function f(): void {}\nclass C {}"),
            "d": ("d", "int f() { return 1; }\nclass C {}"),
            "py": ("py", "def f():\n pass\nclass C:\n pass"),
            "ruby": ("rb", "def f\nend\nclass C\nend"),
            "rust": ("rs", "fn f() {}\nstruct C {}"),
            "kotlin": ("kt", "fun f() {}\nclass C {}"),
            "swift": ("swift", "func f() {}\nstruct C {}"),
            "scala": ("scala", "def f(): Unit = {}\nclass C {}"),
            "php": ("php", "<?php\nfunction f() {}\nclass C {}"),
        }
        for lang, (extension, source) in sources.items():
            # A separate directory makes file/directory --names comparable for the same source.
            directory = root / lang
            directory.mkdir()
            path = directory / ("types with spaces." + extension)
            path.write_text(source, encoding="utf-8")
            base = ["--inp", str(path), "--source", lang]
            same([*base, "--struct", "--map"])
            same([*base, "--struct", "--map", "--json"], True)
            same([*base, "--struct", "--type", "C", "--extract"])
            same([*base, "--struct", "C", "--json"], True)
            same([*base, "--struct", "--type", "absent"], code=2)
            same([*base, "--struct", "--type", "C", "--map"], code=1)
            same([*base, "--all", "--json"], True)
            same([*base, "--all", "--map"])
            same([*base, "--all", "--map", "--extract"])
            for flags in ([], ["--all"], ["--struct"]):
                same([*base, "--names", *flags], c_default=lang == "c" and not flags)
                for workers in (1, 8):
                    same(["--dir", str(directory), "--names", "--workers", str(workers), *flags], c_default=lang == "c" and not flags)
            for flags in (["--struct", "--func", "f"], ["--type", "C", "--map"], ["--struct", "--map", "--lines", "1:2"], ["--all", "--map", "--func", "f"], ["--names", "--json"], ["--names", "--extract=false"]):
                same([*base, *flags], code=1)
    print(f"{checks} type/names CLI comparisons passed")


if __name__ == "__main__":
    main()
