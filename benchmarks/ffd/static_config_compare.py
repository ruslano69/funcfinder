"""Compare frozen D tool builds before/after static language configuration."""
import argparse
import hashlib
import json
import pathlib
import random
import shutil
import statistics
import subprocess
import time


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--before", required=True, help="baseline binary directory")
    ap.add_argument("--after", required=True, help="candidate binary directory")
    ap.add_argument("--out", required=True)
    ap.add_argument("--rounds", type=int, default=31)
    args = ap.parse_args()
    if args.rounds < 1:
        ap.error("--rounds must be positive")
    root = pathlib.Path(__file__).resolve().parents[2]
    out = pathlib.Path(args.out).resolve()
    out.mkdir(parents=True, exist_ok=True)
    empty = out / "empty"
    empty.mkdir(exist_ok=True)
    inputs = {}
    for folder in [root / "internal", root / "test_examples"]:
        for source in sorted(folder.rglob("*")):
            if not source.is_file():
                continue
            relative = source.relative_to(root)
            inputs[str(relative)] = hashlib.sha256(source.read_bytes()).hexdigest()
            target = out / "inputs" / relative
            target.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(source, target)
    directories = {n: pathlib.Path(p).resolve() for n, p in [("before", args.before), ("after", args.after)]}
    cases = {}
    for tool in ["ffd", "funcstats", "callgraph"]:
        cases[tool + "/version"] = (tool, ["--version"])
        for name, path in [("empty", empty), ("internal", root / "internal"), ("fixtures", root / "test_examples")]:
            flags = ["--all", "--json"] if tool == "ffd" else ["-l", "go", "--json"]
            cases[tool + "/" + name] = (tool, ["--dir", str(path), *flags])
        path = str(root / "internal/shardutil.go")
        flags = ["--inp", path, "--source", "go", "--map", "--json"] if tool == "ffd" else (
            [path, "-l", "go", "--json"] if tool == "funcstats" else ["--inp", path, "-l", "go", "--json"])
        cases[tool + "/file"] = (tool, flags)
    samples = {v: {c: [] for c in cases} for v in directories}
    expected = {}
    binaries = {}
    suffix = ".exe" if __import__("os").name == "nt" else ""
    for variant, directory in directories.items():
        binaries[variant] = {}
        for tool in ["ffd", "funcstats", "callgraph"]:
            p = directory / (tool + suffix)
            binaries[variant][tool] = {"path": str(p), "bytes": p.stat().st_size,
                                       "sha256": hashlib.sha256(p.read_bytes()).hexdigest()}

    def run(variant, case, checked=False):
        tool, flags = cases[case]
        command = [binaries[variant][tool]["path"], *flags]
        start = time.perf_counter_ns()
        p = subprocess.run(command, capture_output=True)
        elapsed = (time.perf_counter_ns() - start) / 1e6
        assert p.returncode == 0, (command, p.returncode, p.stderr)
        data = p.stdout if case.endswith("/version") else json.loads(p.stdout)
        if checked:
            assert data == expected[case], (variant, case, "output changed")
        else:
            expected[case] = data
            (out / (case.replace("/", "-") + ".json")).write_bytes(p.stdout)
        return elapsed

    for case in cases:
        run("before", case)
        run("after", case, True)
    for _ in range(2):
        for variant in directories:
            for case in cases:
                run(variant, case, True)
    rng = random.Random(20261021)
    variants = [(v, c) for v in directories for c in cases]
    for _ in range(max(0, args.rounds)):
        rng.shuffle(variants)
        for v, c in variants:
            samples[v][c].append(run(v, c, True))
    medians = {v: {c: statistics.median(s) for c, s in rows.items()} for v, rows in samples.items()}
    for v, tools in binaries.items():
        for metadata in tools.values():
            assert hashlib.sha256(pathlib.Path(metadata["path"]).read_bytes()).hexdigest() == metadata["sha256"]
    for relative, digest in inputs.items():
        assert hashlib.sha256((root / relative).read_bytes()).hexdigest() == digest, relative
    report = {"method": "complete CLI, three warmups, shuffled rounds; identical output required every run",
              "rounds": args.rounds, "binaries": binaries, "inputs": inputs,
              "commands": cases, "samples_ms": samples, "medians_ms": medians}
    (out / "report.json").write_text(json.dumps(report, indent=2), encoding="utf-8")
    for c in cases:
        print(c, "before", round(medians["before"][c], 3), "after", round(medians["after"][c], 3))


if __name__ == "__main__":
    main()
