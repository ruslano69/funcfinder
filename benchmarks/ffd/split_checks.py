"""Verify split manifests, shards, checksums and CLI contracts against Go."""
import argparse
import hashlib
import json
import pathlib
import subprocess
import tempfile


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--ffd", required=True)
    ap.add_argument("--go", required=True)
    args = ap.parse_args()
    go, d = [str(pathlib.Path(p).resolve()) for p in (args.go, args.ffd)]
    checks = 0

    def run(exe, flags, code=0):
        result = subprocess.run([exe, *flags], capture_output=True)
        assert result.returncode == code, (flags, result.returncode, result.stdout, result.stderr)
        return result

    def snapshot(out):
        manifest = json.loads((out / "manifest.json").read_bytes())
        shards = manifest["shards"] or []
        manifest["shards"] = sorted(shards, key=lambda s: s["path"])
        contents = {}
        for s in shards:
            value = json.loads((out / s["path"]).read_bytes())
            value["files"].sort(key=lambda f: f["path"])
            contents[s["path"]] = value
        return manifest, contents

    build = pathlib.Path(__file__).resolve().parents[2] / "build"
    with tempfile.TemporaryDirectory(prefix=".ffd-split-", dir=build) as folder:
        root = pathlib.Path(folder)
        assert root.resolve().parent == build.resolve()
        src = root / "source"
        src.mkdir()
        fixtures = {
            "go": "package p\nfunc f() {}\ntype C struct{}\n",
            "cpp": "int f() {}\nclass C {};\n",
            "c": "int f(void) {}\n",
            "cs": "class C {}\n", "java": "class C {}\n",
            "js": "function f() {}\nclass C {}\n",
            "ts": "function f(): void {}\nclass C {}\n",
            "d": "int f() {}\nclass C {}\n",
            "py": "def f():\n pass\nclass C:\n pass\n",
            "rb": "def f\nend\nclass C\nend\n",
            "rs": "fn f() {}\nstruct C {}\n",
            "kt": "fun f() {}\nclass C {}\n",
            "swift": "func f() {}\nstruct C {}\n",
            "scala": "def f(): Unit = {}\nclass C {}\n",
            "php": "<?php\nfunction f() {}\nclass C {}\n",
        }
        for ext, source in fixtures.items():
            directory = src / ("lang " + ext)
            directory.mkdir()
            (directory / ("пример." + ext)).write_bytes(source.replace("\n", "\r\n").encode())
            (directory / ("empty." + ext)).write_text("\n", encoding="utf-8")
        (src / "root.go").write_text("package p\nfunc root() {}\n", encoding="utf-8")
        empty = src / "empty"
        empty.mkdir()
        (empty / "empty.go").write_text("package p\n", encoding="utf-8")
        alias = src / "aliases"
        alias.mkdir()
        (alias / "alias.go").write_text("package p\ntype A = int\n", encoding="utf-8")
        (src / ".gitignore").write_text("ignored.go\n", encoding="utf-8")
        (src / "ignored.go").write_text("package p\nfunc ignored() {}\n", encoding="utf-8")
        for by in ("dir", "file"):
            for mode in ([], ["--all"], ["--struct"]):
                for workers in (1, 8):
                    for walk in ([], ["--recursive=false"], ["--no-gitignore"]):
                        flags = ["--dir", str(src), "--split", "--split-by", by, "--json", *mode, "--workers", str(workers), *walk]
                        outputs = []
                        for exe, name in ((go, "go"), (d, "d")):
                            out = root / name
                            result = run(exe, [*flags, "--out", str(out)])
                            outputs.append((result.stdout.replace(str(out).encode(), b"OUT"), snapshot(out)))
                        assert outputs[0][0] == outputs[1][0], (flags, outputs[0][0], outputs[1][0])
                        assert outputs[0][1][0] == outputs[1][1][0], (flags, outputs[0][1][0], outputs[1][1][0])
                        for name in outputs[0][1][1]:
                            assert outputs[0][1][1][name] == outputs[1][1][1][name], (flags,name,outputs[0][1][1][name],outputs[1][1][1][name])
                        checks += 1
        # Nonempty files affect checksums even when they hold no declaration; raw content, not normalized CRLF.
        for exe, name in ((go, "go"), (d, "d")):
            out = root / name
            flags = ["--dir", str(src), "--split", "--json", "--all", "--out", str(out)]
            run(exe, flags)
            before = snapshot(out)
            path = src / "lang go" / "empty.go"
            path.write_text("// checksum content " + name, encoding="utf-8")
            run(exe, flags)
            after = snapshot(out)
            old = next(s for s in before[0]["shards"] if s["path"] == "lang go.json")
            new = next(s for s in after[0]["shards"] if s["path"] == "lang go.json")
            assert old["checksum"] != new["checksum"]
            assert before[1]["lang go.json"] == after[1]["lang go.json"]
            path.write_text("\n", encoding="utf-8")
            checks += 1
        for flags, code in ((["--split"], 1), (["--split", "--json", "--split-by=bad"], 1), (["--names", "--split=false"], 1), (["--doc", "f", "--out=x"], 1), (["--split=maybe"], 2)):
            for exe in (go, d):
                run(exe, ["--dir", str(src), *flags, "--out", str(root / "invalid")], code)
            checks += 1
        for by in ("dir", "file"):
            for mode in ([], ["--all"], ["--struct"]):
                targets = [root / ("inc-" + by + "".join(mode) + name) for name in ("go", "d")]
                flags = ["--dir", str(src), "--split", "--json", "--split-by", by, "--inc", *mode]
                for iteration in range(2):
                    outputs = []
                    for exe, out in zip((go, d), targets):
                        result = run(exe, [*flags, "--out", str(out)])
                        outputs.append((result.stdout.replace(str(out).encode(), b"OUT"), snapshot(out)))
                    assert outputs[0] == outputs[1], (by,mode,iteration)
                    checks += 1
        # Empty project uses null shards, and populated repeated D runs are deterministic.
        for exe in (go, d):
            run(exe, ["--dir", str(empty), "--split", "--json", "--out", str(root / "empty-out")])
            assert snapshot(root / "empty-out")[0]["shards"] == []
            checks += 1
        out = root / "deterministic"
        flags = ["--dir", str(src), "--split", "--json", "--out", str(out)]
        run(d, flags)
        digest = hashlib.sha256((out / "manifest.json").read_bytes()).hexdigest()
        run(d, flags)
        assert hashlib.sha256((out / "manifest.json").read_bytes()).hexdigest() == digest
        checks += 1
        # D refuses flattened-name collisions before creating output.
        collision = root / "collision"
        (collision / "a" / "b").mkdir(parents=True)
        (collision / "a_b").mkdir()
        for directory in (collision / "a" / "b", collision / "a_b"):
            (directory / "f.go").write_text("package p\nfunc f() {}\n", encoding="utf-8")
        out = root / "collision-out"
        run(d, ["--dir", str(collision), "--split", "--json", "--out", str(out)], 1)
        assert not out.exists()
        checks += 1
    print(f"{checks} split output/checksum/flag checks passed")


if __name__ == "__main__":
    main()
