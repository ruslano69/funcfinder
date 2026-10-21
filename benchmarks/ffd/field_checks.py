"""Validate token-derived fields with explicit truth; compare supported simple fields with Go."""
import argparse
import json
import pathlib
import subprocess
import tempfile


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--ffd", required=True)
    ap.add_argument("--go", required=True)
    args = ap.parse_args()
    exe, go = (str(pathlib.Path(x).resolve()) for x in (args.ffd, args.go))
    cases = {
        "go": ("go", "package p\ntype C struct {\n Name string\n Count int\n}\n", [("Name", "string", 3), ("Count", "int", 4)]),
        "cpp": ("cpp", "class C {\n public:\n int count;\n double values[4];\n void method() { int local; }\n class Inner { int hidden; };\n // int fake;\n};", [("count", "int", 3), ("values", "double[4]", 4)]),
        "c": ("c", "struct C {\n int count;\n char *name;\n};", [("count", "int", 2), ("name", "char*", 3)]),
        "cs": ("cs", "class C {\n public string Name;\n public int Count { get; set; }\n void Method() { int local; }\n}", [("Name", "string", 2), ("Count", "int", 3)]),
        "java": ("java", "class C {\n private String name;\n public int count = 2;\n void method() { int local; }\n}", [("name", "String", 2), ("count", "int", 3)]),
        "d": ("d", "class C {\n public int count;\n private string name;\n void method() { int local; }\n}", [("count", "int", 2), ("name", "string", 3)]),
        "js": ("js", "class C {\n name = 'text';\n count = 2;\n method() { let local = 1; }\n}", [("name", "", 2), ("count", "", 3)]),
        "ts": ("ts", "class C {\n readonly name: string;\n count?: number = 2;\n method(): void { let local = 1; }\n}", [("name", "string", 2), ("count", "number", 3)]),
        "py": ("py", "class C:\n name: str\n count: int = 2\n def method(self):\n  local: int = 3\n  self.value: float = 1.0", [("name", "str", 2), ("count", "int", 3), ("value", "float", 6)]),
        "rust": ("rs", "struct C {\n pub name: String,\n count: usize,\n}", [("name", "String", 2), ("count", "usize", 3)]),
        "kotlin": ("kt", "class C {\n val name: String = \"text\"\n var count: Int = 2\n fun method() { val local: Int = 3 }\n}", [("name", "String", 2), ("count", "Int", 3)]),
        "swift": ("swift", "struct C {\n var name: String = \"text\"\n let count: Int = 2\n func method() { let local: Int = 3 }\n}", [("name", "String", 2), ("count", "Int", 3)]),
        "scala": ("scala", "class C {\n val name: String = \"text\"\n var count: Int = 2\n def method(): Unit = { val local: Int = 3 }\n}", [("name", "String", 2), ("count", "Int", 3)]),
        "php": ("php", "<?php\nclass C {\n private string $name;\n public int $count = 2;\n function method() { $local = 1; }\n}", [("name", "string", 3), ("count", "int", 4)]),
        "ruby": ("rb", "class C\n text = 'attr_reader :fake'\n attr_reader :name\n attr_accessor :count\n def method\n  local = 1\n end\nend", [("name", "", 3), ("count", "", 4)]),
    }
    build = pathlib.Path(__file__).resolve().parents[2] / "build"
    build.mkdir(exist_ok=True)
    count = 0
    with tempfile.TemporaryDirectory(prefix=".ffd-fields-", dir=build) as folder:
        root = pathlib.Path(folder)
        assert root.resolve().parent == build.resolve()
        for lang, (ext, source, expected) in cases.items():
            path = root / ("fields with spaces." + ext)
            path.write_bytes(source.replace("\n", "\r\n").encode())
            base = ["--inp", str(path), "--source", lang]
            for flags in (["--struct", "--type", "C", "--json"], ["--struct", "--map", "--json"], ["--all", "--json"]):
                p = subprocess.run([exe, *base, *flags], capture_output=True)
                assert not p.returncode, (lang, flags, p.stderr)
                doc = json.loads(p.stdout)
                c = next(t for t in doc["types"] if t["name"] == "C")
                got = [(f["name"], f["type"], f["line"]) for f in c.get("fields", [])]
                assert got == expected, (lang, flags, got, expected)
                count += 1
                if lang == "go":
                    reference = json.loads(subprocess.check_output([go, *base, *flags]))
                    assert c["fields"] == reference["types"][0]["fields"]
                    count += 1
            # Extraction must still use original source, not token spelling.
            p = subprocess.run([exe, *base, "--struct", "--type", "C", "--extract"], capture_output=True, check=True)
            assert b"fields with spaces" not in p.stdout and source.splitlines()[1].encode() in p.stdout
            count += 1
    print(f"{count} field checks passed across {len(cases)} languages")


if __name__ == "__main__":
    main()
