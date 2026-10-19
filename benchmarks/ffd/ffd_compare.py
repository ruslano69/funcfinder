"""Compare ffd (the D port) with the Go funcfinder on whole trees: the keys of the JSON, then the entries by key.

    python benchmarks/ffd/ffd_compare.py --ffd build/ffd/ffd.exe --go funcfinder.exe
    python benchmarks/ffd/ffd_compare.py ... --corpus name=path --mode all --time 5
    python benchmarks/ffd/ffd_compare.py ... --deltas build/ffd/deltas.json
    python benchmarks/ffd/ffd_compare.py ... --expect benchmarks/ffd/ffd_deltas.json [--update]

Both tools run `--dir <path> --<mode> --json` (mode: all, struct or none for functions). The comparison is by key:

  * schema: the key paths of the two documents (files[].path, .functions[].name, ...), with value types;
  * entries: for every file, the multiset of (name, line) of functions and of types. Go's `aliases` are folded into
    the types, as the D finders list aliases among the types;
  * bytes: whether the two outputs are identical.

The differences are the contract: --expect keeps, per pinned corpus (build/*/repos/*) and language, how many entries only Go or only ffd has,
and a hash of the sorted list, so that any change in what differs (better or worse) must be noticed and recorded with
--update. A corpus that is not on this machine is skipped.
"""
import argparse
import collections
import hashlib
import json
import os
import pathlib
import statistics
import subprocess
import sys
import time

HERE = pathlib.Path(__file__).resolve().parent
ROOT = HERE.parent.parent
CONFIG = json.loads((HERE / "ffd_config.json").read_text(encoding="utf-8"))["languages"]


def extension_languages():
    table = {}
    for lang in sorted(CONFIG):
        for ext in CONFIG[lang]["extensions"]:
            if ext not in table or lang == "cpp":
                table[ext] = lang
    return table


EXT = extension_languages()


def language_of(path):
    return EXT.get(os.path.splitext(path)[1], "?")


def default_corpora(roots):
    found = {}
    for repo in sorted(p for r in roots for p in pathlib.Path(r).glob("*/repos/*")):
        if repo.is_dir():
            found.setdefault(repo.name, repo)
    for name, path in (
        ("OmniRoute", r"G:\DEV\OmniRoute"),
        ("funcfinder-internal", str(ROOT / "internal")),
        ("tdtp-framework", r"G:\DEV\Go\tdtp-framework"),
        ("kanzi", r"G:\DEV\Go\kanzi"),
        ("test_examples", str(ROOT / "test_examples")),
    ):
        if pathlib.Path(path).is_dir():
            found.setdefault(name, pathlib.Path(path))
    return {k: str(v) for k, v in found.items()}


def run(exe, directory, mode, workers=0):
    flags = ["--dir", directory, "--json"] + ([f"--{mode}"] if mode != "functions" else [])
    if workers:
        flags += ["--workers", str(workers)]
    start = time.perf_counter()
    p = subprocess.run([exe, *flags], stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    return time.perf_counter() - start, p.returncode, p.stdout, p.stderr.decode("utf-8", "replace")


def key_paths(node, prefix, acc):
    if isinstance(node, dict):
        for k, v in node.items():
            key_paths(v, f"{prefix}.{k}", acc)
    elif isinstance(node, list):
        acc[prefix + "[]"].add("array")
        for v in node:
            key_paths(v, prefix + "[]", acc)
    else:
        acc[prefix].add(type(node).__name__)


def schema(doc):
    acc = collections.defaultdict(set)
    key_paths(doc, "", acc)
    return {k: sorted(v) for k, v in acc.items()}


def entries(doc):
    """{path: (function keys, type keys, alias keys)}, each a Counter of (name, line)."""
    out = {}
    for f in doc["files"]:
        funcs = collections.Counter((e["name"], e["line"]) for e in f.get("functions", []))
        types = collections.Counter((e["name"], e["line"]) for e in f.get("classes", []))
        aliases = collections.Counter((e["name"], e["line"], e["target"]) for e in f.get("aliases", []))
        out[f["path"]] = (funcs, types, aliases)
    return out


def compare(go_doc, d_doc):
    """Per language: counts and the delta list [(kind, which, path, name, line)]."""
    g, d = entries(go_doc), entries(d_doc)
    stats = collections.defaultdict(lambda: collections.Counter())
    deltas = []
    empty = (collections.Counter(), collections.Counter(), collections.Counter())
    for path in sorted(set(g) | set(d)):
        lang = language_of(path)
        gf, gt, ga = g.get(path, empty)
        df, dt, da = d.get(path, empty)
        for kind, a, b in (("function", gf, df), ("type", gt, dt), ("alias", ga, da)):
            stats[lang][kind + "s_go"] += sum(a.values())
            stats[lang][kind + "s_ffd"] += sum(b.values())
            stats[lang][kind + "s_match"] += sum((a & b).values())
            for (name, line), n in (a - b).items():
                stats[lang][kind + "_only_go"] += n
                deltas += [(kind, "only_go", path, name, line)] * n
            for (name, line), n in (b - a).items():
                stats[lang][kind + "_only_ffd"] += n
                deltas += [(kind, "only_ffd", path, name, line)] * n
    return stats, deltas


def digest(deltas, root):
    # Paths relative to the corpus root, so the hash does not depend on where the corpus lives.
    rows = sorted((k, w, os.path.relpath(p, root).replace("\\", "/"), n, ln) for k, w, p, n, ln in deltas)
    return hashlib.sha256(json.dumps(rows, ensure_ascii=False).encode("utf-8")).hexdigest()[:16], rows


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--ffd", required=True)
    ap.add_argument("--go", required=True)
    ap.add_argument("--corpus", action="append", default=[], help="name=path (default: the corpora of this machine)")
    ap.add_argument("--mode", default="all", choices=["all", "struct", "functions"])
    ap.add_argument("--time", type=int, default=0, help="N timed runs of each tool, interleaved (0: no timing)")
    ap.add_argument("--workers", type=int, default=0)
    ap.add_argument("--deltas", help="write every difference to this JSON file")
    ap.add_argument("--expect", help="expected differences (ffd_deltas.json)")
    ap.add_argument("--update", action="store_true", help="with --expect: write the current differences as expected")
    ap.add_argument("--only", help="comma-separated corpus names")
    ap.add_argument("--corpora-root", action="append", default=[], help="a build directory whose */repos/* are corpora (default: build/ of this checkout)")
    a = ap.parse_args()
    a.ffd, a.go = os.path.abspath(a.ffd), os.path.abspath(a.go)

    corpora = dict(c.split("=", 1) for c in a.corpus) if a.corpus else default_corpora(a.corpora_root or [ROOT / "build"])
    if a.only:
        corpora = {k: v for k, v in corpora.items() if k in a.only.split(",")}
    # The reference is whatever --go is: say which version, and keep it with the recorded differences, so that results
    # measured against another build (a stale local binary, an older release) cannot pass for these.
    reference = subprocess.run([a.go, "--version"], capture_output=True).stdout.decode("utf-8", "replace").strip()
    print(f"reference: {reference}")
    expected = json.loads(pathlib.Path(a.expect).read_text(encoding="utf-8")) if a.expect and not a.update and pathlib.Path(a.expect).exists() else {}
    current, all_deltas, failures = {}, {}, 0

    for name, path in corpora.items():
        if not pathlib.Path(path).is_dir():
            print(f"{name}: not on this machine, skipped")
            continue
        _, grc, gout, gerr = run(a.go, path, a.mode, a.workers)
        _, drc, dout, derr = run(a.ffd, path, a.mode, a.workers)
        if grc or drc:
            print(f"{name}: exit codes go={grc} ffd={drc}\n  go: {gerr[-300:]}\n  ffd: {derr[-300:]}")
            failures += 1
            continue
        gdoc, ddoc = json.loads(gout), json.loads(dout)
        gs, ds = schema(gdoc), schema(ddoc)
        schema_diff = sorted(set(gs) ^ set(ds)) + sorted(k for k in set(gs) & set(ds) if gs[k] != ds[k] and "int" not in gs[k] + ds[k])
        stats, deltas = compare(gdoc, ddoc)
        digest_value, rows = digest(deltas, path)
        identical = gout == dout
        total_go = sum(s["functions_go"] for s in stats.values())
        total_match = sum(s["functions_match"] for s in stats.values())
        only_go = sum(s["function_only_go"] for s in stats.values())
        only_ffd = sum(s["function_only_ffd"] for s in stats.values())
        tgo = sum(s["types_go"] for s in stats.values())
        tonly_go = sum(s["type_only_go"] for s in stats.values())
        tonly_ffd = sum(s["type_only_ffd"] for s in stats.values())
        line = (f"{name:22} {'IDENTICAL' if identical else 'differs  '} functions: Go {total_go:6} match {total_match:6} "
                f"only-Go {only_go:4} only-ffd {only_ffd:4} | types: Go {tgo:5} only-Go {tonly_go:4} only-ffd {tonly_ffd:4}"
                f"{' | SCHEMA ' + str(schema_diff) if schema_diff else ''}")
        if a.time:
            times = {"go": [], "ffd": []}
            for _ in range(a.time):
                times["go"].append(run(a.go, path, a.mode, a.workers)[0])
                times["ffd"].append(run(a.ffd, path, a.mode, a.workers)[0])
            mg, md = statistics.median(times["go"]) * 1000, statistics.median(times["ffd"]) * 1000
            line += f" | {mg:7.0f} -> {md:7.0f} ms ({100 * (md - mg) / mg:+.0f}%)"
        print(line, flush=True)
        all_deltas[name] = rows
        if schema_diff:
            failures += 1
        if os.sep + "repos" + os.sep not in path.replace("/", os.sep):
            continue  # only pinned corpora (build/*/repos/*) take part in --expect: the other trees change
        current[name] = {"schema": schema_diff, "digest": digest_value, "languages": {l: dict(s) for l, s in sorted(stats.items())}}

    if a.deltas:
        pathlib.Path(a.deltas).write_text(json.dumps(all_deltas, ensure_ascii=False), encoding="utf-8")
    if a.expect:
        slim = {n: {"digest": c["digest"], "schema": c["schema"],
                    "languages": {l: {k: v for k, v in s.items() if k.endswith(("_only_go", "_only_ffd"))} for l, s in c["languages"].items()
                                  if any(k.endswith(("_only_go", "_only_ffd")) for k in s)}} for n, c in current.items()}
        if a.update:
            slim["_reference"] = reference
            pathlib.Path(a.expect).write_text(json.dumps(slim, indent=1, sort_keys=True) + "\n", encoding="utf-8")
            print(f"wrote {a.expect}")
        else:
            if expected.get("_reference") != reference:
                print(f"REFERENCE DIFFERS: differences were recorded against {expected.get('_reference')!r}, now {reference!r}")
                failures += 1
            for n, c in slim.items():
                if n in expected and expected[n] != c:
                    print(f"UNEXPECTED DIFFERENCES in {n}: expected {expected[n]['languages']} digest {expected[n]['digest']}, "
                          f"now {c['languages']} digest {c['digest']}")
                    failures += 1
                elif n not in expected:
                    print(f"{n}: no expectation recorded (run with --update)")
                    failures += 1
    sys.exit(1 if failures else 0)


if __name__ == "__main__":
    main()
