"""Compare file/directory --doc output and flag contracts with the Go CLI."""
import argparse
import pathlib
import subprocess
import tempfile


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--ffd", required=True)
    ap.add_argument("--go", required=True)
    args = ap.parse_args()
    d, go = (str(pathlib.Path(p).resolve()) for p in (args.ffd, args.go))
    count = 0

    def same(flags, code=0):
        nonlocal count
        a, b = [subprocess.run([exe, *flags], capture_output=True) for exe in (go, d)]
        assert a.returncode == b.returncode == code and a.stdout == b.stdout, (flags, a.returncode, b.returncode, a.stdout, b.stdout, a.stderr, b.stderr)
        count += 1

    # Declarations are fieldless: this checks doc selection/formatting separately from native parser profile deltas.
    fixtures = {
        "go": ("go", "package p\n// Документация f\nfunc f() {\n println(1)\n}\n/* C docs */\ntype C struct{}\n// A docs\ntype A = C"),
        "cpp": ("cpp", "/**\n * f docs\n */\nint f() {\n return 1;\n}\n// C docs\nclass C {};"),
        "c": ("c", "// f docs\nint f(void) {\n return 1;\n}\n// C docs\nstruct C {};"),
        "cs": ("cs", "// C docs\nclass C {\n /// f docs\n int f() {\n return 1;\n }\n}"),
        "java": ("java", "// C docs\nclass C {\n /** f docs */\n int f() {\n return 1;\n }\n}"),
        "js": ("js", "// f docs\nfunction f() {\n return 1;\n}\n// C docs\nclass C {}"),
        "ts": ("ts", "// f docs\nfunction f(): number {\n return 1;\n}\n// C docs\nclass C {}"),
        "d": ("d", "// f docs\nint f() {\n return 1;\n}\n// C docs\nclass C {}"),
        "py": ("py", "# f docs\n@decorator\ndef f():\n return 1\n# C docs\nclass C:\n pass"),
        "ruby": ("rb", "# f docs\ndef f\n 1\nend\n# C docs\nclass C\nend"),
        "rust": ("rs", "/// f docs\n#[inline]\nfn f() {\n}\n/// C docs\nstruct C {}"),
        "kotlin": ("kt", "// f docs\nfun f(): Int {\n return 1\n}\n// C docs\nclass C {}"),
        "swift": ("swift", "// f docs\nfunc f() -> Int {\n return 1\n}\n// C docs\nstruct C {}"),
        "scala": ("scala", "// f docs\ndef f(): Int = {\n 1\n}\n// C docs\nclass C {}"),
        "php": ("php", "<?php\n// f docs\nfunction f() {\n return 1;\n}\n// C docs\nclass C {}"),
    }
    build = pathlib.Path(__file__).resolve().parents[2] / "build"
    build.mkdir(exist_ok=True)
    with tempfile.TemporaryDirectory(prefix=".ffd-doc-", dir=build) as folder:
        root = pathlib.Path(folder)
        assert root.resolve().parent == build.resolve()
        for lang, (ext, source) in fixtures.items():
            directory = root / lang
            directory.mkdir()
            path = directory / ("docs with spaces." + ext)
            path.write_bytes(source.replace("\n", "\r\n").encode())
            base = ["--inp", str(path), "--source", lang]
            for names in ("f", "C", " C, f,missing ", "missing"):
                code = 2 if names == "missing" else 0
                same([*base, "--doc", names], code)
                for workers in (1, 8):
                    same(["--dir", str(directory), "--doc", names, "--workers", str(workers)], code)
            for flags in (["--map=false"], ["--extract=false"], ["--json"], ["--names"], ["--all"], ["--struct"], ["--lines", "1:2"], ["--func", "f"], ["--type", "C"]):
                same([*base, "--doc", "f", *flags], 1)
            same([*base, "--doc", " , "], 1)
            if lang == "go":
                same([*base, "--doc", "A"])
                same(["--dir", str(directory), "--doc", "A"])
        extras = [
            ("go", "// detached\n\nfunc f() {}"),
            ("go", "// first\n/* second */\n// third\nfunc f() {\n panic(\"HIDDEN\")\n}"),
            ("cpp", "// first overload\nint f() {}\n// second overload\nint f(int x) {}"),
            ("rs", "/// docs\n#[cfg(\n test\n)]\nfn f() {}"),
            ("py", "# docs\n@decorator(\n 1\n)\ndef f():\n pass"),
            ("py", "def f():\n \"\"\"HIDDEN DOCSTRING\"\"\"\n pass"),
        ]
        for i, (ext, source) in enumerate(extras):
            lang = {"rs": "rust"}.get(ext, ext)
            path = root / f"extra-{i}.{ext}"
            path.write_text(source, encoding="utf-8")
            same(["--inp", str(path), "--source", lang, "--doc", "f"])
            p = subprocess.run([d, "--inp", str(path), "--source", lang, "--doc", "f"], capture_output=True, check=True)
            assert b"HIDDEN" not in p.stdout
        same(["--doc"], 2)
        same(["--inp", str(root / "missing.go"), "--source", "go", "--doc", "f"], 1)
    print(f"{count} documentation CLI comparisons passed")


if __name__ == "__main__":
    main()
