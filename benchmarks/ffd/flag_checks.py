"""Audit Go/D CLI keys, boolean values and argument boundaries on a controlled fixture."""
import argparse
import pathlib
import re
import subprocess
import tempfile


TRUE_VALUES = ("1", "t", "T", "true", "TRUE", "True")
FALSE_VALUES = ("0", "f", "F", "false", "FALSE", "False")


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--ffd", required=True)
    ap.add_argument("--go", required=True)
    args = ap.parse_args()
    go, d = [str(pathlib.Path(p).resolve()) for p in (args.go, args.ffd)]

    def flags(exe):
        result = subprocess.run([exe, "--help"], capture_output=True)
        assert result.returncode == 0 and not result.stdout, (exe,result)
        return dict((name, kind or b"bool") for name, kind in re.findall(rb"^  -(\S+?)(?: (string|int))?$", result.stderr, re.M))

    expected, actual = flags(go), flags(d)
    assert expected.items() <= actual.items(), (expected,actual)
    assert set(actual) - set(expected) == {b"timing", b"timing-alloc"}
    checks = 1
    for name, kind in actual.items():
        values = ["0"] if kind == b"int" else ["value"] if kind == b"string" else TRUE_VALUES + FALSE_VALUES
        for value in values:
            result = subprocess.run([d,"--" + name.decode() + "=" + value,"--version"],capture_output=True)
            assert result.returncode == 0, (name,value,result.stderr)
            checks += 1
    for spelling in ("--help", "-h", "-help", "--help=false", "--h"):
        result = subprocess.run([d,spelling],capture_output=True)
        assert result.returncode == 0 and b"Usage of ffd:" in result.stderr
        checks += 1

    def compare(argv, *, version=False, code=0):
        nonlocal checks
        before, after = [subprocess.run([exe, *argv], capture_output=True) for exe in (go, d)]
        assert before.returncode == after.returncode == code, (argv, before, after)
        if code == 0 and not version:
            assert (before.stdout, before.stderr) == (after.stdout, after.stderr), (argv, before, after)
        checks += 1

    # --version isolates parsing, while fixture output below verifies actual boolean state.
    # Go's --struct preprocessor treats equals values as type names (even false/0),
    # so it is not a standard boolean reference. Keep D's explicit false semantics.
    for name, kind in expected.items():
        if kind != b"bool" or name == b"struct":
            continue
        for prefix in ("-", "--"):
            for value in TRUE_VALUES + FALSE_VALUES:
                compare([prefix + name.decode() + "=" + value, "--version"], version=True)
            for value in ("", "yes", "2", "TrUe", " true"):
                compare([prefix + name.decode() + "=" + value, "--version"], code=2)

    work = pathlib.Path(__file__).resolve().parents[2] / "build"
    work.mkdir(exist_ok=True)
    with tempfile.TemporaryDirectory(prefix="flag-checks-", dir=work) as temp:
        source = pathlib.Path(temp) / "fixture.go"
        source.write_text("package fixture\ntype Example struct {}\nfunc First() {}\nfunc Second() {}\n", encoding="utf-8")
        base = ["--inp", str(source), "--source", "go", "--map"]
        for value in TRUE_VALUES + FALSE_VALUES:
            compare([*base, "--json=" + value])
            compare([*base, "--json", "--json=" + value])
            # Explicitly check the intentional struct-preprocessor difference.
            reference_args = [*base, "--struct"] if value in TRUE_VALUES else base
            before = subprocess.run([go, *reference_args], capture_output=True)
            after = subprocess.run([d, *base, "--struct=" + value], capture_output=True)
            assert (before.returncode, before.stdout, before.stderr) == (after.returncode, after.stdout, after.stderr), (value, before, after)
            checks += 1
        # Do not parse help, unknown flags, selectors or explicit-false flags past a stop.
        for stop in ("--", "-", "extra"):
            for tail in ([], ["--unknown"], ["--help"], ["--func", "Missing"], ["--json=false"]):
                compare([*base, "--json", stop, *tail])
            compare(["--version", stop, "--unknown"], version=True)
            compare([stop, "--version"], code=1)
        compare([*base[:-1], "--names", "--", "--raw=false"])
        compare([*base[:-1], "--names", "--raw=false", "--"], code=1)
        compare([*base, "--unknown", "--"], code=2)
        compare([*base, "--json", "false", "--unknown"])
        # Go's shorthand preprocessor drops empty standalone arguments; D preserves the boundary.
        before = subprocess.run([go, *base, "--json"], capture_output=True)
        after = subprocess.run([d, *base, "--json", "", "--unknown"], capture_output=True)
        dropped = subprocess.run([go, *base, "--json", "", "--unknown"], capture_output=True)
        assert dropped.returncode == 2 and before.returncode == after.returncode == 0, (before, after, dropped)
        assert (before.stdout, before.stderr) == (after.stdout, after.stderr), (before, after)
        checks += 1
        # -- is a value when consumed by a string flag, not an argument boundary.
        compare([*base[:-1], "--func", "--"], code=2)
    print(f"{len(expected)} Go keys covered; D adds timing/timing-alloc; {checks} key/help/behavior checks passed")


if __name__ == "__main__":
    main()
