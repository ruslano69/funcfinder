"""Check incremental D output against fresh D scans and unchanged-file preservation."""
import argparse
import json
import pathlib
import subprocess
import tempfile


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--ffd", required=True)
    args = ap.parse_args()
    exe = str(pathlib.Path(args.ffd).resolve())
    build = pathlib.Path(__file__).resolve().parents[2] / "build"
    checks = 0
    with tempfile.TemporaryDirectory(prefix=".ffd-inc-", dir=build) as folder:
        root = pathlib.Path(folder)
        assert root.resolve().parent == build.resolve()
        src = root / "src"
        for by in ("dir", "file"):
            for directory in (src, src / "a", src / "b", src / "empty"):
                directory.mkdir(exist_ok=True)
            (src / "root.go").write_text("package p\nfunc root() {}\n", encoding="utf-8")
            (src / "a" / "a.go").write_text("package p\nfunc a() {}\ntype A struct{}\n", encoding="utf-8")
            (src / "a" / "blank.go").write_text("package p\n", encoding="utf-8")
            (src / "b" / "b.go").write_text("package p\nfunc b() {}\n", encoding="utf-8")
            (src / "empty" / "e.go").write_text("package p\n", encoding="utf-8")
            out = root / ("inc-" + by)
            full = root / ("full-" + by)

            def run(target, inc=True, mode=None):
                flags = [exe, "--dir", str(src), "--json", "--split", "--split-by", by, "--out", str(target), "--workers", "8", *(mode or [])]
                if inc:
                    flags.append("--inc")
                result = subprocess.run(flags, capture_output=True)
                assert result.returncode == 0, (flags, result.stderr)
                return result

            def snapshot(target):
                manifest = json.loads((target / "manifest.json").read_bytes())
                manifest["shards"] = sorted(manifest["shards"] or [], key=lambda s: s["path"])
                shards = {s["path"]: json.loads((target / s["path"]).read_bytes()) for s in manifest["shards"]}
                return manifest, shards

            def check(mode=None):
                nonlocal checks
                result = run(out, mode=mode)
                run(full, inc=False, mode=mode)
                assert snapshot(out) == snapshot(full), (by, mode, snapshot(out), snapshot(full))
                checks += 1
                return result

            check()
            before = {p.name: (p.read_bytes(), p.stat().st_mtime_ns) for p in out.glob("*.json") if p.name != "manifest.json"}
            result = check()
            assert b"0 shards changed" in result.stderr
            assert before == {name: ((out / name).read_bytes(), (out / name).stat().st_mtime_ns) for name in before}
            checks += 1
            # A changed declaration and an empty-source change both invalidate their shard.
            (src / "a" / "a.go").write_text("package p\nfunc changed() {}\ntype A struct{}\n", encoding="utf-8")
            check()
            (src / "empty" / "e.go").write_text("// empty bytes changed\n", encoding="utf-8")
            result = check()
            assert b"1 shards changed" in result.stderr
            # A populated shard becomes empty, then nonempty again.
            (src / "b" / "b.go").write_text("package p\n", encoding="utf-8")
            check()
            (src / "b" / "b.go").write_text("package p\nfunc back() {}\n", encoding="utf-8")
            check()
            # Full deletion must drop the old shard from both disk and manifest.
            name = "b.json" if by == "dir" else "b_b_go.json"
            (src / "b" / "b.go").unlink()
            check()
            assert not (out / name).exists()
            checks += 1
            (src / "b" / "new.go").write_text("package p\nfunc added() {}\n", encoding="utf-8")
            check()
            # Mode changes and missing/corrupt artifacts must trigger repair.
            check(["--all"])
            check(["--struct"])
            listed = {s["path"] for s in snapshot(out)[0]["shards"]}
            assert {p.name for p in out.glob("*.json") if p.name != "manifest.json"} == listed
            check()
            path = next(p for p in out.glob("*.json") if p.name != "manifest.json")
            path.unlink()
            check()
            path = next(p for p in out.glob("*.json") if p.name != "manifest.json")
            path.write_text("broken", encoding="utf-8")
            check()
            (out / "manifest.json").write_text("broken", encoding="utf-8")
            check()
            (out / ".ffd-state").unlink()
            check()
            run(out, inc=False)
            assert not (out / ".ffd-state").exists()
            check()
            # No source files left: totals/empty_shards and owned shard files must clear.
            for path in src.rglob("*.go"):
                path.unlink()
            check()
            assert not [p for p in out.glob("*.json") if p.name != "manifest.json"]
            checks += 1
    print(f"{checks} incremental invalidation/repair/fresh-scan checks passed")


if __name__ == "__main__":
    main()
