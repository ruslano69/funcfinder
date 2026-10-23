"""Export independent Kotlin PSI objects; optionally run native JSON-edit checks.

Uses an existing portable runtime containing jdk-*/ and kotlinc/lib/. Does not
download tools or execute Gradle/application code. All generated files go to --out.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import subprocess


def run(args, **kwargs):
    return subprocess.check_output(list(map(str, args)), encoding="utf-8",
                                   stderr=subprocess.STDOUT, **kwargs)


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--runtime", type=Path, required=True)
    parser.add_argument("--root", type=Path, action="append", required=True)
    parser.add_argument("--out", type=Path, required=True)
    parser.add_argument("--check", action="store_true", help="run TestKotlinCodeObjectsCorpus")
    args = parser.parse_args()
    repo = Path(__file__).resolve().parents[1]
    work = args.out.resolve()
    work.mkdir(parents=True, exist_ok=True)
    runtime = args.runtime.resolve()
    jdks = list(runtime.glob("jdk-*"))
    if len(jdks) != 1:
        parser.error("--runtime must contain exactly one jdk-* directory")
    suffix = ".exe" if os.name == "nt" else ""
    java, javac = [jdks[0] / "bin" / (name + suffix) for name in ("java", "javac")]
    libraries = str(runtime / "kotlinc/lib/*")
    source = work / "KotlinObjectsOracle.java"
    source.write_bytes((repo / "benchmarks/kotlin_objects_oracle.java.txt").read_bytes())
    classes = work / "classes"
    classes.mkdir(exist_ok=True)
    build = [javac, "-proc:none", "-cp", libraries, "-d", classes, source]
    (work / "oracle-build.log").write_text(run(build), encoding="utf-8")
    paths, pins = [], {}
    for root in dict.fromkeys(p.resolve() for p in args.root):
        git = ["git", "-c", f"safe.directory={root.as_posix()}", "-C", root]
        pins[str(root)] = run(git + ["rev-parse", "HEAD"]).strip()
        paths.extend(str(root / rel) for rel in run(git + ["ls-files", "-z", "*.kt", "*.kts"]).split("\0") if rel)
    if not paths:
        parser.error("no tracked Kotlin files in the supplied roots")
    manifest = work / "manifest.txt"
    manifest.write_text("\n".join(paths) + "\n", encoding="utf-8")
    command = [java, "-cp", str(classes) + os.pathsep + libraries, "KotlinObjectsOracle", manifest]
    output = run(command)
    (work / "oracle.jsonl").write_text(output, encoding="utf-8")
    files = [json.loads(line) for line in output.splitlines() if line.startswith("{")]
    if len(files) != len(paths) or not any(f["objects"] for f in files):
        raise RuntimeError("incomplete or empty PSI reference")
    version = run([java, "-cp", libraries, "org.jetbrains.kotlin.cli.jvm.K2JVMCompiler", "-version"]).strip()
    overlay = work / "source-snapshot"
    overlay.mkdir(exist_ok=True)
    hashes = {}
    for relative in ["internal/kotlin_objects.go", "internal/token_code_edits.go", "internal/code_object.go",
                     "internal/code_edits.go", "internal/kotlin_code_objects_corpus_test.go",
                     "benchmarks/kotlin_objects_oracle.java.txt", "benchmarks/kotlin_objects_eval.py"]:
        target = overlay / relative
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_bytes((repo / relative).read_bytes())
        hashes[relative] = digest(target)
    implementation = {
        "base_revision": run(["git", "-C", repo, "rev-parse", "HEAD"]).strip(),
        "profile": "working-tree overlay; file/type declaration lists; code-token spans excluding edge comments/trivia",
        "sources": hashes, "build_command": list(map(str, build)), "command": list(map(str, command)),
        "oracle_class_sha256": digest(classes / "KotlinObjectsOracle.class"),
        "compiler_jars": {p.name: digest(p) for p in sorted((runtime / "kotlinc/lib").glob("*.jar"))},
        "java_sha256": digest(java), "javac_sha256": digest(javac),
    }
    oracle = work / "oracle.json"
    oracle.write_text(json.dumps({"kotlin": version, "root": str(args.root), "revision": json.dumps(pins),
                                 "implementation": implementation, "files": files}, ensure_ascii=False, indent=2), encoding="utf-8")
    print(f"{len(files)} files; {sum(len(f['objects']) for f in files)} objects; "
          f"{sum(bool(f['errors']) for f in files)} PSI-error exclusions", flush=True)
    if args.check:
        env = dict(os.environ, FUNC_KOTLIN_ORACLE=str(oracle), FUNC_KOTLIN_REPORT=str(work / "report.json"))
        result = subprocess.run(["go", "test", "./internal", "-run", "TestKotlinCodeObjectsCorpus", "-count=1", "-v"],
                                cwd=repo, env=env, capture_output=True, text=True, encoding="utf-8")
        (work / "check.log").write_text(result.stdout + result.stderr, encoding="utf-8")
        print(result.stdout + result.stderr, end="")
        if result.returncode:
            raise SystemExit(result.returncode)


if __name__ == "__main__":
    main()
