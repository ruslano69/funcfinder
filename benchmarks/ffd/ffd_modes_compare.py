"""Compare the modes of ffd with the Go funcfinder, byte for byte: --names, --doc, --tree, --tree-full and --split on whole
trees, and the single-file views (--func, --map, --tree, --tree-full, --extract, --json, --struct, --all) on files of
every language.

    python benchmarks/ffd/ffd_modes_compare.py --ffd build/ffd/ffd.exe --go <release funcfinder> --corpora-root <repo>/build
    python benchmarks/ffd/ffd_modes_compare.py ... --files 4 --only py,go --show 3

The two tools use different finders, so some outputs differ by what they find (the differences recorded in
ffd_deltas.json): a view is "same" only when finders and formatting agree. Formatting is held to the Go tool by the
trees where the finders agree (`internal`, a Go tree, is byte-identical in --names, --tree and --doc but for the order of
the types); the rest of the report tells how far each mode is from Go on real code. The reference must be the release
binary (see AGENTS.md), and its --version is printed.
"""
import argparse
import collections
import glob
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile

EXT = {"c": [".c"], "cpp": [".cpp", ".cc", ".hpp", ".h"], "cs": [".cs"], "d": [".d"], "go": [".go"], "java": [".java"],
       "js": [".js"], "kotlin": [".kt"], "php": [".php"], "py": [".py"], "ruby": [".rb"], "rust": [".rs"], "scala": [".scala"],
       "swift": [".swift"], "ts": [".ts"]}
FILE_COMBOS = [
    ["--map"], ["--map", "--json"], ["--tree"], ["--tree-full"], ["--map", "--extract"],
    ["--struct", "--map"], ["--struct", "--tree"], ["--struct", "--map", "--json"], ["--struct", "--extract"],
    ["--all", "--map"], ["--all", "--tree"], ["--all", "--json"], ["--all", "--extract"],
    ["--map", "--lines", "20:140"], ["--map", "--extract", "--lines", "60:200"], ["--map", "--raw"], ["--struct", "--map", "--raw"],
]
DIR_COMBOS = [["--names"], ["--all", "--names"], ["--struct", "--names"], ["--tree"], ["--all", "--tree-full"], ["--struct", "--tree"]]


def run(exe, args, cwd=None):
    p = subprocess.run([exe, *args], capture_output=True, cwd=cwd)
    return p.returncode, p.stdout


def first_diff(a, b):
    la, lb = a.decode("utf-8", "replace").split("\n"), b.decode("utf-8", "replace").split("\n")
    for i in range(max(len(la), len(lb))):
        x = la[i] if i < len(la) else "<missing>"
        y = lb[i] if i < len(lb) else "<missing>"
        if x != y:
            return f"line {i + 1}: go  {x[:110]}\n{' ' * (len(str(i + 1)) + 6)}ffd {y[:110]}"
    return "same text"


def pick_files(root, lang, count):
    found = []
    for ext in EXT[lang]:
        found += glob.glob(os.path.join(root, "**", "*" + ext), recursive=True)
    found = [f for f in found if os.path.isfile(f) and 300 < os.path.getsize(f) < 120000 and "/." not in f.replace(os.sep, "/")]
    found.sort(key=lambda f: -os.path.getsize(f))
    # spread over the size range: the largest, then evenly down
    step = max(1, len(found) // max(1, count))
    return found[::step][:count]


def main():
    sys.stdout.reconfigure(encoding="utf-8")
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--ffd", required=True)
    ap.add_argument("--go", required=True)
    ap.add_argument("--corpora-root", required=True, help="the checkout's build/ directory (corpora are build/*/repos/*)")
    ap.add_argument("--files", type=int, default=3, help="files per language for the single-file views")
    ap.add_argument("--only", help="comma-separated language keys")
    ap.add_argument("--show", type=int, default=2, help="differences shown per view")
    ap.add_argument("--no-dirs", action="store_true")
    ap.add_argument("--no-files", action="store_true")
    ap.add_argument("--no-split", action="store_true")
    a = ap.parse_args()
    go, ffd = os.path.abspath(a.go), os.path.abspath(a.ffd)
    langs = a.only.split(",") if a.only else list(EXT)
    print("reference:", subprocess.run([go, "--version"], capture_output=True, text=True).stdout.strip())

    shown = collections.Counter()
    tally = collections.defaultdict(lambda: [0, 0])

    def compare(group, label, args):
        gc, gout = run(go, args)
        fc, fout = run(ffd, args)
        ok = gc == fc and gout == fout
        tally[group][1] += 1
        if ok:
            tally[group][0] += 1
        elif shown[group] < a.show:
            shown[group] += 1
            print(f"  DIFF {group}: {label} (exit go={gc} ffd={fc})\n    {first_diff(gout, fout)}")
        return ok

    corpora = sorted(glob.glob(os.path.join(a.corpora_root, "*", "repos", "*")))
    if not a.no_files:
        for lang in langs:
            files = []
            for c in corpora:
                files += pick_files(c, lang, 1)
            files = files[: a.files] if len(files) >= a.files else files
            for f in files:
                base = ["--inp", f, "--source", lang]
                # the first function name for --func
                _, out = run(go, base + ["--map"])
                m = re.match(rb"\s*([^:;\s][^:;]*): \d+-\d+", out)
                combos = [(" ".join(c), c) for c in FILE_COMBOS]
                combos += [("--lines", ["--lines", "5:12"]), ("--lines --json", ["--lines", "5:12", "--json"])]
                if m:
                    name = m.group(1).decode()
                    combos += [("--func", ["--func", name]), ("--func --extract", ["--func", name, "--extract"])]
                for suffix, combo in combos:
                    standalone = combo[0] == "--lines"  # --lines alone reads plain lines, no language
                    compare(f"file/{lang}/{suffix}", os.path.basename(f) + " " + " ".join(combo), (["--inp", f] if standalone else base) + combo)
    if not a.no_dirs:
        for c in corpora + [os.path.join(os.path.dirname(a.corpora_root), "internal")]:
            if not os.path.isdir(c):
                continue
            for combo in DIR_COMBOS:
                compare("dir/" + " ".join(combo), os.path.basename(c) + " " + " ".join(combo), ["--dir", c] + combo)
    if not a.no_split:
        # The manifest is compared apart from the counts (the finders differ): shard names, checksums, empty shards and
        # the header must be Go's; the shard files are compared as a whole and differ where the finders do.
        for c in corpora[:6]:
            for by in ("dir", "file"):
                outs = []
                for exe in (go, ffd):
                    d = tempfile.mkdtemp()
                    subprocess.run([exe, "--dir", c, "--json", "--split", "--split-by", by, "--out", d], capture_output=True)
                    state, manifest = {}, {}
                    for p in sorted(glob.glob(os.path.join(d, "*.json"))):
                        text = open(p, encoding="utf-8").read()
                        if os.path.basename(p) == "manifest.json":
                            m = json.loads(text)
                            manifest = {"version": m["version"], "root_dir": m["root_dir"], "split_by": m["split_by"],
                                        "shards": sorted((s_["path"], s_["checksum"]) for s_ in (m["shards"] or [])),
                                        "empty": m.get("empty_shards")}
                        else:
                            state[os.path.basename(p)] = text
                    shutil.rmtree(d)
                    outs.append((manifest, state))
                for group, same, detail in (
                        (f"split/{by}/manifest (names, checksums, header)", outs[0][0] == outs[1][0], "manifest differs"),
                        (f"split/{by}/shard files", outs[0][1] == outs[1][1],
                         f"{len([k for k in set(outs[0][1]) | set(outs[1][1]) if outs[0][1].get(k) != outs[1][1].get(k)])} shard files differ")):
                    tally[group][1] += 1
                    if same:
                        tally[group][0] += 1
                    elif shown[group] < a.show:
                        shown[group] += 1
                        print(f"  DIFF {group}: {os.path.basename(c)}: {detail}")

    print()
    width = max(len(k) for k in tally) if tally else 10
    for k in sorted(tally):
        same, total = tally[k]
        print(f"{k:<{width}}  {same:>4}/{total:<4} identical")


if __name__ == "__main__":
    main()
