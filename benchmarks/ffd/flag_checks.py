"""Audit advertised Go/D CLI key coverage and bool/value parsing without scanning files."""
import argparse
import pathlib
import re
import subprocess


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--ffd", required=True)
    ap.add_argument("--go", required=True)
    args = ap.parse_args()
    go, d = [str(pathlib.Path(p).resolve()) for p in (args.go, args.ffd)]

    def flags(exe):
        result = subprocess.run([exe, "--help"], capture_output=True)
        assert result.returncode == 0 and not result.stdout, (exe,result)
        return dict((name, kind or "bool") for name, kind in re.findall(rb"^  -(\S+?)(?: (string|int))?$", result.stderr, re.M))

    expected, actual = flags(go), flags(d)
    assert expected.items() <= actual.items(), (expected,actual)
    assert set(actual) - set(expected) == {b"timing", b"timing-alloc"}
    checks = 1
    for name, kind in actual.items():
        values = ["0"] if kind == b"int" else ["value"] if kind == b"string" else ["true", "false", "1", "0"]
        for value in values:
            result = subprocess.run([d,"--" + name.decode() + "=" + value,"--version"],capture_output=True)
            assert result.returncode == 0, (name,value,result.stderr)
            checks += 1
    for spelling in ("--help", "-h", "-help", "--help=false", "--h"):
        result = subprocess.run([d,spelling],capture_output=True)
        assert result.returncode == 0 and b"Usage of ffd:" in result.stderr
        checks += 1
    print(f"{len(expected)} Go keys covered; D adds timing/timing-alloc; {checks} key/help checks passed")


if __name__ == "__main__":
    main()
