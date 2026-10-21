"""Check Python parsed --lines adjustment, reports and extraction against the Go CLI."""
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
    fixtures = [
        "# global\nx = 1\n\n@first\ndef f():\n    x = 1\n    return x\n\n# trivia\ndef g(): return 2\n\n",
        "class C:\n    @staticmethod\n    def f():\n        def inner():\n            return 1\n        return inner()\n\n    def g(self):\n        return 2\n\nx = 3\n",
        "@decorator(\n    option=(1, 2),\n)\nasync def f(\n    value: dict[str, int],\n):\n    text = '''def fake():\nno dedent here\n'''\n    return value\n\nasync def g(): pass",
        "def f():\n\tvalue = (\n0 +\n1\n)\n\treturn value\n\nclass C:\n\tdef g(self):\n\t\tpass\n",
        "# global\n\nclass Empty: pass\n\ndef f(): return 'x'\n# tail\n",
        "value = '''\ndef fake(): pass\n'''\n# still no scopes\n",
        "@декоратор\ndef функция():\n    return 'Привет'\n\nclass Класс:\n    def метод(self): pass\n",
    ]
    build = pathlib.Path(__file__).resolve().parents[2] / "build"
    build.mkdir(exist_ok=True)
    count = 0
    with tempfile.TemporaryDirectory(prefix=".ffd-pyrange-", dir=build) as folder:
        root = pathlib.Path(folder)
        assert root.resolve().parent == build.resolve()
        for index, source in enumerate(fixtures):
            path = root / (f"range {index}.py")
            path.write_bytes(source.replace("\n", "\r\n").encode("utf-8"))
            total = len(source.splitlines())
            ranges = {"1:", f"1:{total}", ":2", f"{total}:999", f"{total+2}:", str(total+1), f"{total+1}:"}
            for line in range(1, total+1):
                ranges.add(str(line))
                ranges.add(f"{line}:")
            for range_ in sorted(ranges):
                for flags in (["--map"], ["--map", "--json"], ["--map", "--extract"], ["--func", "f,inner", "--extract"]):
                    command = ["--inp", str(path), "--source", "py", "--lines", range_, *flags]
                    a, b = [subprocess.run([exe, *command], capture_output=True) for exe in (go, d)]
                    assert a.returncode == b.returncode and a.stdout == b.stdout, (index, range_, flags, a.returncode, b.returncode, a.stdout, b.stdout, a.stderr, b.stderr)
                    if not a.returncode:
                        assert a.stderr == b.stderr, (index, range_, flags, a.stderr, b.stderr)
                    count += 1
    print(f"{count} Python range CLI comparisons passed")


if __name__ == "__main__":
    main()
