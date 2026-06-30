#!/usr/bin/env python3
"""specsheet.py — produce one honest spec-sheet row for a reference project.

Measures funcfinder against the AST ruler (cmd/astoracle for Go) and computes:
  - accuracy:  recall + precision of symbols vs the parser ground truth,
               matched by (name, line +/-2) within each file
  - tokens:    token savings of the funcfinder map vs reading raw source

Time-to-comprehension and task-success are deliberately NOT here: they require
an agent-in-the-loop runner (phase 2). This file only emits the two columns
that are fully automatable and reproducible.

Usage:
  python specsheet.py --funcfinder ff.json --oracle oracle.json \\
                      --root <dir> --label "tdtp (Go)" [--sha <commit>]
"""
import argparse
import json
import os
import sys

LINE_TOL = 2  # a symbol counts as "found" if name matches and line is within +/-2


def norm(p: str) -> str:
    return p.replace("\\", "/").lstrip("./")


def load(path):
    with open(path, encoding="utf-8") as f:
        return json.load(f)


def index_by_file(doc):
    """path -> {'functions': [(name,line)], 'classes': [(name,line)]}"""
    out = {}
    for fobj in doc.get("files", []):
        key = norm(fobj["path"])
        out[key] = {
            "functions": [(s["name"], s.get("line", 0)) for s in fobj.get("functions") or []],
            "classes": [(s["name"], s.get("line", 0)) for s in fobj.get("classes") or []],
        }
    return out


def match_count(truth, cand):
    """Greedy: how many `truth` symbols have a same-name cand within LINE_TOL.
    Each cand is consumed once so duplicates aren't double-counted."""
    used = [False] * len(cand)
    found = 0
    for tname, tline in truth:
        best = -1
        bestd = LINE_TOL + 1
        for i, (cname, cline) in enumerate(cand):
            if used[i] or cname != tname:
                continue
            d = abs(cline - tline)
            if d <= LINE_TOL and d < bestd:
                best, bestd = i, d
        if best >= 0:
            used[best] = True
            found += 1
    return found


def count_tokens(text):
    """Prefer tiktoken (real BPE); fall back to chars/4 with a flag."""
    try:
        import tiktoken
        enc = tiktoken.get_encoding("cl100k_base")
        return len(enc.encode(text)), "tiktoken/cl100k_base"
    except Exception:
        return max(1, len(text) // 4), "chars/4 (approx)"


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--funcfinder", required=True)
    ap.add_argument("--oracle", required=True)
    ap.add_argument("--root", required=True)
    ap.add_argument("--label", required=True)
    ap.add_argument("--sha", default="(unpinned)")
    ap.add_argument("--map-text", help="path to the raw funcfinder map output (for token sizing); "
                                       "defaults to the --funcfinder JSON file")
    args = ap.parse_args()

    ff = index_by_file(load(args.funcfinder))
    orc = index_by_file(load(args.oracle))

    ff_files = set(ff)
    orc_files = set(orc)
    both = ff_files & orc_files
    only_oracle = orc_files - ff_files   # files funcfinder produced no symbols for
    only_ff = ff_files - orc_files        # files the oracle couldn't parse / didn't see

    # Symbol-level recall/precision over the file intersection, so file-selection
    # differences don't silently corrupt the accuracy number.
    orc_total = ff_total = matched_recall = matched_prec = 0
    misses = []  # (path, kind, name, line) the oracle has but funcfinder missed
    for f in both:
        for kind in ("functions", "classes"):
            t = orc[f][kind]
            c = ff[f][kind]
            orc_total += len(t)
            ff_total += len(c)
            mr = match_count(t, c)
            matched_recall += mr
            matched_prec += match_count(c, t)
            if mr < len(t):
                # record which symbols were missed (name set difference, approx)
                cnames = {n for n, _ in c}
                for n, ln in t:
                    if n not in cnames:
                        misses.append((f, kind, n, ln))

    recall = matched_recall / orc_total if orc_total else 1.0
    precision = matched_prec / ff_total if ff_total else 1.0

    # Token savings: tokens of the map vs tokens of the raw .go source it replaces.
    map_path = args.map_text or args.funcfinder
    with open(map_path, encoding="utf-8") as f:
        map_text = f.read()
    map_tok, tokenizer = count_tokens(map_text)

    raw_tok = 0
    for f in orc_files | ff_files:
        fp = os.path.join(args.root, f)
        try:
            with open(fp, encoding="utf-8", errors="ignore") as fh:
                raw_tok += count_tokens(fh.read())[0]
        except OSError:
            pass
    token_savings = (1 - map_tok / raw_tok) if raw_tok else 0.0

    print(f"\n=== spec-sheet row: {args.label}  @ {args.sha} ===")
    print(f"  tokenizer:        {tokenizer}")
    print(f"  files (oracle/ff/both): {len(orc_files)}/{len(ff_files)}/{len(both)}")
    print(f"  symbols (oracle/ff):    {orc_total}/{ff_total}")
    print(f"  ACCURACY  recall:    {recall*100:.1f}%   (oracle symbols funcfinder found)")
    print(f"  ACCURACY  precision: {precision*100:.1f}%   (funcfinder symbols that are real)")
    print(f"  TOKEN SAVINGS:       {token_savings*100:.1f}%   (map {map_tok:,} tok vs raw {raw_tok:,} tok)")
    if only_oracle:
        print(f"  ! files only the oracle saw (funcfinder found 0 symbols): {len(only_oracle)}")
        for f in sorted(only_oracle)[:5]:
            print(f"      {f}")
    if only_ff:
        print(f"  ! files only funcfinder saw (oracle parse error / skipped): {len(only_ff)}")
    print(f"  misses (oracle-has, funcfinder-missed): {len(misses)}")
    for f, kind, n, ln in misses[:12]:
        print(f"      {f}:{ln}  {kind[:-1]} {n}")
    print("  NOTE: time-to-comprehension & task-success are phase 2 (need agent runner).")


if __name__ == "__main__":
    main()
