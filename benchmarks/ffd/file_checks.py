"""Verify ffd file CLI contracts against Go and original source bytes (no external packages).

python benchmarks/ffd/file_checks.py --ffd build/ffd/ffd.exe --go funcfinder.exe
"""
import argparse
import json
import pathlib
import subprocess
import tempfile


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--ffd", required=True)
    parser.add_argument("--go", required=True)
    args = parser.parse_args()
    d, go = (str(pathlib.Path(p).resolve()) for p in (args.ffd, args.go))
    checks = 0

    def run(exe, flags):
        return subprocess.run([exe, *flags], capture_output=True)

    def same(flags, *, json_output=False, code=0):
        nonlocal checks
        a, b = run(go, flags), run(d, flags)
        assert a.returncode == b.returncode == code, (flags, a.returncode, b.returncode, a.stderr, b.stderr)
        if code == 0:
            assert (json.loads(a.stdout) == json.loads(b.stdout) if json_output else a.stdout == b.stdout), (flags, a.stdout, b.stdout)
        checks += 1

    build = pathlib.Path(__file__).resolve().parents[2] / "build"
    build.mkdir(exist_ok=True)
    with tempfile.TemporaryDirectory(prefix=".ffd-file-", dir=build) as folder:
        root = pathlib.Path(folder)
        assert root.resolve().parent == build.resolve()
        fixtures = {
            "go": ("go", "package p\nfunc f() {\n println(1)\n}\nfunc g() {}"),
            "cpp": ("cpp", "struct C {\n int f() & { return 1; }\n int g() noexcept(false) { return 2; }\n};"),
            "cs": ("cs", "class C {\n int f() { return 1; }\n int g() { return 2; }\n}"),
            "java": ("java", "class C {\n int f() { return 1; }\n int g() { return 2; }\n}"),
            "js": ("js", "function f() { return 1; }\nfunction g() { return 2; }"),
            "ts": ("ts", "function f(): number { return 1; }\nfunction g(): number { return 2; }"),
            "c": ("c", "int f(void) { return 1; }\nint g(void) { return 2; }"),
            "d": ("d", "int f() { return 1; }\nint g() { return 2; }"),
            "py": ("py", "@first\ndef f():\n return 1\ndef g():\n return 2"),
            "ruby": ("rb", "def f\n 1\nend\ndef g\n 2\nend"),
            "rust": ("rs", "fn f() -> i32 { 1 }\nfn g() -> i32 { 2 }"),
            "kotlin": ("kt", "fun f(): Int { return 1 }\nfun g(): Int { return 2 }"),
            "swift": ("swift", "func f() -> Int { return 1 }\nfunc g() -> Int { return 2 }"),
            "scala": ("scala", "def f(): Int = { 1 }\ndef g(): Int = { 2 }"),
            "php": ("php", "<?php\nfunction f() { return 1; }\nfunction g() { return 2; }"),
        }
        for lang, (extension, source) in fixtures.items():
            path = root / ("source with spaces." + extension)
            path.write_bytes(source.replace("\n", "\r\n").encode())
            base = ["--inp", str(path), "--source", lang]
            same([*base, "--map"])
            same([*base, "--map", "--json"], json_output=True)
            same([*base, "--func", "f", "--extract"])
            same([*base, "--func", "f,g", "--json"], json_output=True)
            same([*base, "--func", "absent"], code=2)
            same([*base, "--map", "--func", "f"], code=1)
            same([*base], code=1)
        go_path = root / "source with spaces.go"
        base = ["--inp", str(go_path), "--source", "go"]
        same([*base, "--map", "--lines", "2:4", "--json"], json_output=True)
        same([*base, "--func", "f", "--lines", "2:4", "--extract"])
        raw = root / "text.any"
        raw.write_bytes('a\t"\\\r\nПривет\r\n\r\nlast'.encode())
        for range_ in ("1", ":2", "2:", ":", "1:99", "3:3"):
            same(["--inp", str(raw), "--lines", range_])
            same(["--inp", str(raw), "--lines", range_, "--json"], json_output=True)
        for range_ in ("0", "2:1", "1:0", "1:-1", "x", "1:2:3", "99:"):
            same(["--inp", str(raw), "--lines", range_], code=1)
        same(["--inp", str(raw), "--map"], code=1)
        same(["--inp", str(raw), "--source", "unknown", "--map"], code=1)
        same(["--inp", str(root / "missing.go"), "--source", "go", "--map"], code=1)
        raw.write_bytes(b"raw\xff\x00\r\nlast")
        same(["--inp", str(raw), "--lines", "1:2"])
        # The original body must survive non-ASCII text and a line above Scanner's old 64 KiB limit.
        source = 'package p\nfunc f() {\n println("Привет ' + "x" * 131072 + '")\n}'
        go_path.write_bytes(source.encode())
        same([*base, "--func", "f", "--extract"])
        overloads = root / "overloads.cpp"
        overloads.write_text("int f(int x) { return x; }\nint f() { return 1; }", encoding="utf-8")
        same(["--inp", str(overloads), "--source", "cpp", "--func", "f", "--json"], json_output=True)
        same(["--inp", str(overloads), "--source", "cpp", "--func", "f", "--extract"])
        for flags in (["--source"], ["--extract=maybe"]):
            same(flags, code=2)
    print(f"{checks} file CLI comparisons passed")


if __name__ == "__main__":
    main()
