"""Score Go-source call graphs with the native AST and time complete CLI runs.

Use a Git source subtree (e.g. --root internal). Tracked and nonignored untracked
Go files form the AST scope; scope mismatches with the CLI remain explicit.
"""
import argparse
import hashlib
import json
import os
import pathlib
import random
import statistics
import subprocess
import time


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--root", required=True)
    ap.add_argument("--d", required=True)
    ap.add_argument("--go", required=True)
    ap.add_argument("--out", required=True)
    ap.add_argument("--time", type=int, default=7)
    a = ap.parse_args()
    root, out = pathlib.Path(a.root).resolve(), pathlib.Path(a.out).resolve()
    binaries = {name: pathlib.Path(p).resolve() for name, p in [("d", a.d), ("go", a.go)]}
    out.mkdir(parents=True, exist_ok=True)

    def run(args, **kwargs):
        p = subprocess.run(list(map(str, args)), capture_output=True, **kwargs)
        if p.returncode:
            raise RuntimeError((args, p.returncode, p.stderr.decode(errors="replace")))
        return p.stdout

    paths = run(["git", "-C", root, "ls-files", "-z", "--cached", "--others", "--exclude-standard", "--", "*.go"]).split(b"\0")
    files = sorted({root / os.fsdecode(p) for p in paths if p and not any(part.startswith(".") for part in pathlib.Path(os.fsdecode(p)).parts)})
    if not files:
        raise RuntimeError("no Git-visible Go source files in scope")
    source_hash = hashlib.sha256()
    for f in files:
        raw = f.read_bytes()
        source_hash.update(str(f.relative_to(root)).encode())
        source_hash.update(b"\0")
        source_hash.update(raw)
        saved = out / "sources" / f.relative_to(root)
        saved.parent.mkdir(parents=True, exist_ok=True)
        saved.write_bytes(raw)
    manifest = {
        "root": str(root), "revision": run(["git", "-C", root, "rev-parse", "HEAD"]).decode().strip(),
        "source_files": len(files), "source_sha256": source_hash.hexdigest(),
        "go_version": run(["go", "version"]).decode().strip(),
        "cpu_count": os.cpu_count(), "workers": "D default 8, Go runtime CPU default",
        "profile": "named syntax calls; immediate receiver normalization; computed receivers retain known bare names; no type resolution",
    }
    oracle_source = out / "oracle.go"
    oracle_source.write_bytes(pathlib.Path(__file__).with_name("callgraph_go_ast.go.txt").read_bytes())
    manifest["oracle_sha256"] = hashlib.sha256(oracle_source.read_bytes()).hexdigest()
    oracle = out / ("oracle.exe" if os.name == "nt" else "oracle")
    env = dict(os.environ, GOCACHE=str(out / "go-cache"))
    run(["go", "build", "-o", oracle, oracle_source], env=env)
    truth_bytes = run([oracle], input=json.dumps(list(map(str, files))).encode())
    (out / "ast.json").write_bytes(truth_bytes)

    def edges(payload):
        data = json.loads(payload)
        return {(str(pathlib.Path(f["path"]).relative_to(root)).replace("\\", "/"), e["caller"], e["callee"], e["line"])
                for f in data["files"] or [] for e in f["calls"] or []}

    truth = edges(truth_bytes)
    report = dict(manifest, ast_edges=len(truth), variants={})
    initial = {}
    for name, exe in binaries.items():
        (out / (name + "-callgraph" + exe.suffix)).write_bytes(exe.read_bytes())
        payload = run([exe, "--dir", root, "-l", "go", "--json"])
        (out / f"{name}.json").write_bytes(payload)
        initial[name] = json.loads(payload)
        actual = edges(payload)
        report["variants"][name] = {
            "binary": str(exe), "bytes": exe.stat().st_size,
            "sha256": hashlib.sha256(exe.read_bytes()).hexdigest(),
            "version": run([exe, "--version"]).decode().strip(),
            "edges": len(actual), "missing": sorted(truth - actual), "extra": sorted(actual - truth),
            "samples_ms": [],
        }
    # Timing follows scoring. Both processes are warmed; every run must preserve
    # that variant's own output, even if variants intentionally differ.
    rng = random.Random(20261021)
    for _ in range(max(0, a.time)):
        names = list(binaries)
        rng.shuffle(names)
        for name in names:
            start = time.perf_counter()
            payload = run([binaries[name], "--dir", root, "-l", "go", "--json"])
            elapsed = (time.perf_counter() - start) * 1000
            assert json.loads(payload) == initial[name], f"unstable graph: {name}"
            report["variants"][name]["samples_ms"].append(elapsed)
    for name, item in report["variants"].items():
        assert hashlib.sha256(binaries[name].read_bytes()).hexdigest() == item["sha256"], "binary changed during measurement"
        if item["samples_ms"]:
            item["median_ms"] = statistics.median(item["samples_ms"])
        print(name, item["bytes"], "bytes;", item["edges"], "edges; missing", len(item["missing"]),
              "extra", len(item["extra"]), "median", item.get("median_ms"))
    for f in files:
        assert f.read_bytes() == (out / "sources" / f.relative_to(root)).read_bytes(), "source changed during measurement"
    (out / "report.json").write_text(json.dumps(report, ensure_ascii=False, indent=2), encoding="utf-8")


if __name__ == "__main__":
    main()
