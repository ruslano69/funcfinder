"""Build a D-only finder API port and compare it with the production Go API.

This serial prototype covers function/class bounds, not the five-tool CLI.
Artifacts, manifests, toolchains and generated Go sources stay under build/.
"""
import argparse
import hashlib
import importlib.metadata
import json
import os
import pathlib
import platform
import random
import statistics
import subprocess
import sys
import tarfile
import time

GO_PROBE = r'''
package main
import (
 "encoding/json"
 "fmt"
 "os"
 "strconv"
 "strings"
 "time"
 ff "github.com/ruslano69/funcfinder/internal"
)
func main() {
 cfg,err:=ff.LoadConfig(); if err!=nil {panic(err)}
 c,err:=cfg.GetLanguageConfig("d"); if err!=nil {panic(err)}
 if len(os.Args)>1 && os.Args[1]=="--config" {
  json.NewEncoder(os.Stdout).Encode(map[string]any{"func_pattern":c.FuncRegex().String(),"class_pattern":c.ClassRegex().String(),"exclude_words":c.ExcludeWords});return
 }
 if len(os.Args)<4 {panic("usage: go-probe config manifest repeats [--time]")}
 data,err:=os.ReadFile(os.Args[2]); if err!=nil {panic(err)}
 var paths []string; if err=json.Unmarshal(data,&paths);err!=nil {panic(err)}
 corpus:=make([][]string,len(paths))
 for i,p:=range paths {lines,e:=ff.ReadLines(p);if e!=nil {panic(e)};corpus[i]=lines}
 finder:=ff.NewFinder(c,nil,true,false,false)
 timing:=len(os.Args)>4 && os.Args[4]=="--time"
 if timing {for _,lines:=range corpus {if _,err=finder.FindFunctionsInLines(lines,1,"");err!=nil {panic(err)}}}
 repeats,err:=strconv.Atoi(os.Args[3]);if err!=nil {panic(err)}
 var checksum int;results:=make([]*ff.FindResult,len(paths));start:=time.Now()
 for n:=0;n<repeats;n++ {for i,lines:=range corpus {r,e:=finder.FindFunctionsInLines(lines,1,"");if e!=nil {panic(e)};checksum+=len(r.Functions)+len(r.Classes);results[i]=r}}
 elapsed:=time.Since(start).Nanoseconds()
 enc:=json.NewEncoder(os.Stdout)
 if timing {enc.Encode(map[string]any{"nanoseconds":elapsed,"checksum":checksum});return}
 for i,r:=range results {fs:=make([][]any,0,len(r.Functions));cs:=make([][]any,0,len(r.Classes));for _,f:=range r.Functions {fs=append(fs,[]any{f.Name,f.Start,f.End,f.ClassName})};for _,c:=range r.Classes {cs=append(cs,[]any{c.Name,c.Start,c.End})};if err=enc.Encode(map[string]any{"path":strings.ReplaceAll(paths[i],"\\","/"),"functions":fs,"classes":cs});err!=nil {panic(err)}}
 fmt.Fprint(os.Stderr,"")
}
'''


def run(command, **kwargs):
    result = subprocess.run(list(map(str, command)), capture_output=True, text=True,
                            encoding="utf-8", errors="replace", **kwargs)
    if result.returncode:
        raise RuntimeError(f"{command}: exit {result.returncode}\n{result.stdout}\n{result.stderr}")
    return result.stdout


def digest(path):
    return hashlib.sha256(pathlib.Path(path).read_bytes()).hexdigest()


def oracle(maps, module_dir, grammar_cache):
    sys.path.insert(0, str(pathlib.Path(module_dir).resolve()))
    import tree_sitter_language_pack as pack
    from scala_d_tree_eval import symbols
    cache = pathlib.Path(grammar_cache).resolve()
    pack.configure(pack.PackConfig(cache_dir=str(cache), languages=["d"]))
    parser = pack.get_parser("d")
    counts = dict(files=0, unparseable=0, truth=0, found=0, missing=0, false_positive=0, wrong_end=0, oracle_rejected=0)
    findings = []
    for row in maps:
        path = pathlib.Path(row["path"])
        source = path.read_bytes()
        tree = parser.parse(source)
        if tree.root_node.has_error:
            counts["unparseable"] += 1
            findings.append({"file": str(path), "kind": "unparseable"})
            continue
        rejected = []
        truth = symbols(tree, source, "d", rejected)
        counts["oracle_rejected"] += len(rejected)
        counts["files"] += 1
        counts["truth"] += len(truth)
        used = set()
        for node in truth:
            matches = [(i, f) for i, f in enumerate(row["functions"]) if i not in used and f[0] == node["name"] and node["start"] <= f[1] <= node["name_line"]]
            if not matches:
                counts["missing"] += 1
                findings.append({"file": str(path), "kind": "missing", **node})
                continue
            i, candidate = min(matches, key=lambda item: abs(item[1][1] - node["start"]))
            used.add(i)
            counts["found"] += 1
            if candidate[2] != node["end"]:
                counts["wrong_end"] += 1
                findings.append({"file": str(path), "kind": "wrong_end", **node, "candidate": candidate})
        for i, candidate in enumerate(row["functions"]):
            if i not in used:
                counts["false_positive"] += 1
                findings.append({"file": str(path), "kind": "false_positive", "candidate": candidate})
    dll = next(cache.rglob("tree_sitter_d.dll"))
    return {"counts": counts, "findings": findings,
            "tree_sitter": importlib.metadata.version("tree-sitter"),
            "language_pack": importlib.metadata.version("tree-sitter-language-pack"),
            "grammar_sha256": digest(dll)}


def measure(args):
    repo = pathlib.Path(__file__).resolve().parent.parent
    work = pathlib.Path(args.work_dir).resolve()
    work.mkdir(parents=True, exist_ok=True)
    if not work.is_relative_to(repo / "build"):
        raise ValueError("--work-dir must be inside this repository's ignored build/")
    env = dict(os.environ, GOPROXY="off", GOAMD64="v1", GOCACHE=str(pathlib.Path(os.environ["TEMP"]) / "funcfinder-go-cache"))
    revision = run(["git", "rev-parse", args.revision + "^{commit}"], cwd=repo).strip()
    reference = work / "reference-src"
    pin_file = work / "reference-revision.txt"
    if pin_file.exists() and pin_file.read_text().strip() != revision:
        raise RuntimeError("reference revision changed; choose a fresh work directory")
    if not pin_file.exists():
        archive = work / "reference.tar"
        run(["git", "archive", "--format=tar", "--output", archive, revision], cwd=repo)
        reference.mkdir(exist_ok=True)
        with tarfile.open(archive) as reader:
            reader.extractall(reference, filter="data")
        pin_file.write_text(revision, encoding="utf-8")
    source = reference / "build/go_probe.go"
    source.parent.mkdir(exist_ok=True)
    source.write_text(GO_PROBE, encoding="utf-8")
    go_binary = work / "go-probe.exe"
    d_binary = work / "d-probe.exe"
    d_ct_binary = work / "d-ct-probe.exe"
    go_command = ["go", "build", "-ldflags=-s -w", "-o", go_binary, source]
    run(go_command, cwd=reference, env=env)
    config = work / "config.json"
    config.write_text(run([go_binary, "--config"], cwd=repo, env=env), encoding="utf-8")
    patterns = json.loads(config.read_text())
    (work / "dlang_port_patterns.d").write_text("module dlang_port_patterns;\n" +
        "enum functionPatternText = " + json.dumps(patterns["func_pattern"]) + ";\n" +
        "enum classPatternText = " + json.dumps(patterns["class_pattern"]) + ";\n", encoding="utf-8")
    d_command = [args.ldc, "-O3", "-release", "-mcpu=x86-64", f"-of={d_binary}", repo / "benchmarks/dlang_port.d"]
    ct_command = [args.ldc, "-O3", "-release", "-mcpu=x86-64", "-d-version=CompiledPatterns", f"-I={work}", f"-of={d_ct_binary}", repo / "benchmarks/dlang_port.d"]
    build_bat = work / "compile-d.bat"
    build_bat.write_text('@echo off\ncall "' + str(pathlib.Path(args.env_bat)) + '" >nul\nif errorlevel 1 exit /b 1\n' + subprocess.list2cmdline(list(map(str, d_command))) + '\nexit /b %errorlevel%\n', encoding="utf-8")
    build_command = ["cmd", "/d", "/c", str(build_bat)]
    build_output = run(build_command, cwd=repo, env=env)
    (work / "d-build.txt").write_text(build_output, encoding="utf-8")
    build_bat.write_text('@echo off\ncall "' + str(pathlib.Path(args.env_bat)) + '" >nul\nif errorlevel 1 exit /b 1\n' + subprocess.list2cmdline(list(map(str, ct_command))) + '\nexit /b %errorlevel%\n', encoding="utf-8")
    (work / "d-ct-build.txt").write_text(run(build_command, cwd=repo, env=env), encoding="utf-8")
    report = {"revision": revision,
              "dirty": bool(run(["git", "status", "--porcelain"], cwd=repo).strip()),
              "platform": platform.platform(), "python": platform.python_version(),
              "go": run(["go", "version"]).strip(),
              "go_amd64": "v1", "logical_cpus": os.cpu_count(),
              "ldc": run([args.ldc, "--version"]).split("Registered Targets:")[0].strip(),
              "build_commands": [list(map(str, cmd)) for cmd in [go_command,d_command,ct_command]],
              "profile": "serial finder API: function name/start/end/scope and raw class name/start/end; no CLI feature parity claim",
              "scope": "Go probe links the production generic finder; D probe implements only D. Size ratio includes this specialization.",
              "binaries": {k: {"path": str(p), "bytes": p.stat().st_size, "sha256": digest(p)}
                           for k, p in [("go", go_binary), ("d", d_binary), ("d_ct",d_ct_binary)]},
              "sources": {p: digest((reference if p.startswith("internal/") else repo) / p) for p in ["benchmarks/dlang_port.d", "benchmarks/dlang_port_compare.py", "internal/d_lines.go", "internal/finder.go", "internal/languages.json"]},
              "samples": args.samples, "inner_repeats": args.repeats, "seed": 20261018, "projects": []}
    binaries = {"go": go_binary, "d": d_binary, "d_ct": d_ct_binary}
    if args.baseline_report:
        baseline_path = pathlib.Path(args.baseline_report).resolve()
        baseline = json.loads(baseline_path.read_text(encoding="utf-8"))
        for name in ["go", "d", "d_ct"]:
            info = baseline["binaries"][name]
            path = pathlib.Path(info["path"])
            if digest(path) != info["sha256"]:
                raise RuntimeError(f"baseline {name} binary changed")
            binaries[name + "_previous"] = path
            report["binaries"][name + "_previous"] = info
        if json.loads(run([binaries["go_previous"], "--config"], cwd=repo, env=env)) != patterns:
            raise RuntimeError("baseline configuration differs; cannot reuse compiled patterns")
        report["baseline"] = {"report": str(baseline_path), "report_sha256": digest(baseline_path),
                              "revision": baseline["revision"], "sources": baseline["sources"]}
    rng = random.Random(20261018)
    for root_arg in args.root:
        root = pathlib.Path(root_arg).resolve()
        label = root.name
        paths = run(["git", "-C", root, "ls-files", "*.d"]).splitlines()
        absolute = [str(root / p).replace("\\", "/") for p in paths]
        manifest = work / f"{label}-manifest.json"
        manifest.write_text(json.dumps(absolute, ensure_ascii=False), encoding="utf-8")
        commands = {k: [str(p), str(config), str(manifest), "1"] for k, p in binaries.items()}
        maps = {}
        for name, command in commands.items():
            output = run(command, cwd=repo, env=env)
            (work / f"{label}-{name}.jsonl").write_text(output, encoding="utf-8")
            maps[name] = [json.loads(line) for line in output.splitlines()]
        # Arrays retain duplicates and original result order, including class scope.
        mismatches = [{"file": paths[i], "variant": name,"go": a, "d": b} for name in commands if name != "go" for i, (a, b) in enumerate(zip(maps["go"], maps[name])) if a != b]
        if any(len(m) != len(paths) for m in maps.values()):
            raise RuntimeError("file result count mismatch")
        project = {"root": str(root), "pin": run(["git", "-C", root, "rev-parse", "HEAD"]).strip(),
                   "dirty": bool(run(["git", "-C", root, "status", "--porcelain"]).strip()),
                   "files": len(paths), "bytes": sum((root / p).stat().st_size for p in paths),
                   "functions": sum(len(r["functions"]) for r in maps["go"]),
                   "classes": sum(len(r["classes"]) for r in maps["go"]),
                   "parity": not mismatches, "mismatches": mismatches}
        report["projects"].append(project)
        (work / "report.json").write_text(json.dumps(report, indent=2), encoding="utf-8")
        print(json.dumps({k: v for k, v in project.items() if k != "mismatches"}), flush=True)
        if mismatches:
            raise RuntimeError(f"{label}: {len(mismatches)} file mismatches; timing blocked")
        quality = oracle(maps["d_ct"], args.module_dir, args.grammar_cache)
        project["oracle"] = quality
        (work / "report.json").write_text(json.dumps(report, indent=2), encoding="utf-8")
        print(json.dumps({"project": label, "oracle": quality["counts"]}), flush=True)
        if any(quality["counts"][k] for k in ["missing", "false_positive", "wrong_end"]):
            raise RuntimeError("independent quality check failed; timing blocked")
        if args.samples == 0:
            continue
        samples = {k: {"parser_ms": [], "cli_ms": []} for k in commands}
        for _ in range(2):
            for command in commands.values():
                run(command, cwd=repo, env=env)
        for _ in range(args.samples):
            order = list(commands)
            rng.shuffle(order)
            for name in order:
                command = commands[name]
                timed = json.loads(run(command[:-1] + [str(args.repeats), "--time"], cwd=repo, env=env))
                if timed["checksum"] != (project["functions"] + project["classes"]) * args.repeats:
                    raise RuntimeError("timed parser result count changed")
                samples[name]["parser_ms"].append(timed["nanoseconds"] / args.repeats / 1e6)
                start = time.perf_counter()
                output = run(command, cwd=repo, env=env)
                samples[name]["cli_ms"].append((time.perf_counter() - start) * 1000)
                if [json.loads(line) for line in output.splitlines()] != maps[name]:
                    raise RuntimeError("timed CLI output changed")
        project["timings"] = {name: {kind: {"median": statistics.median(values), "min": min(values), "max": max(values), "samples": values}
                                           for kind, values in timing.items()} for name, timing in samples.items()}
        (work / "report.json").write_text(json.dumps(report, indent=2), encoding="utf-8")
        print(json.dumps({"project": label, "timings": project["timings"]}), flush=True)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", action="append", required=True)
    parser.add_argument("--revision", default="v1.15.3")
    parser.add_argument("--work-dir", default="build/dlang-port-20261018")
    parser.add_argument("--ldc", default="F:/D/ldc-1.43.0/bin/ldc2.exe")
    parser.add_argument("--env-bat", default="F:/D/ldcvars64.bat")
    parser.add_argument("--samples", type=int, default=10, help="Shuffled timing rounds; 0 runs only parity and quality checks")
    parser.add_argument("--repeats", type=int, default=2)
    parser.add_argument("--module-dir", default="build/d-eval-runtime")
    parser.add_argument("--grammar-cache", default="build/d-grammar-cache")
    parser.add_argument("--baseline-report", help="Reuse hash-verified previous binaries in the same shuffled rounds")
    measure(parser.parse_args())
