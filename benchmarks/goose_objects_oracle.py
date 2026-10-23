"""Export exact top-level Goose object spans using an upstream Tree-sitter build.

Requires a caller-supplied tree-sitter runtime and grammar library. Does not
install dependencies, run application code, or treat grammar acceptance as
compiler/type/lifetime validation. Nested functions and extern members are
outside the initial JSON editor's scope, but remain navigation targets.
"""
import argparse
import ctypes
import hashlib
import importlib.metadata
import json
from pathlib import Path
import re
import subprocess
import sys


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    for flag in ("root", "runtime", "grammar-library", "output"):
        ap.add_argument("--" + flag, required=True)
    args = ap.parse_args()
    root = Path(args.root).resolve()
    sys.path.insert(0, str(Path(args.runtime).resolve()))
    from tree_sitter import Language, Parser

    library = Path(args.grammar_library).resolve()
    lib = ctypes.CDLL(str(library))
    lib.tree_sitter_goose.restype = ctypes.c_void_p
    capsule = ctypes.pythonapi.PyCapsule_New
    capsule.restype = ctypes.py_object
    capsule.argtypes = [ctypes.c_void_p, ctypes.c_char_p, ctypes.c_void_p]
    parser = Parser(Language(capsule(lib.tree_sitter_goose(), b"tree_sitter.Language", None)))
    tracked = subprocess.check_output(["git", "-C", str(root), "ls-files", "-z"]).decode("utf-8").split("\0")
    kinds = {"function_declaration": "function", "struct_declaration": "type",
             "enum_declaration": "type", "type_alias_declaration": "type"}
    files = []
    for rel in tracked:
        if not rel.endswith(".goose") or not rel.startswith(("stdlib/", "samples/", "bench/", "test/")) or rel.startswith(("test/errors/", "test/errors_tc/")):
            continue
        path = root / rel
        data = path.read_bytes()
        tree = parser.parse(data)
        entry = {"path": str(path), "sha256": hashlib.sha256(data).hexdigest(), "objects": []}
        if tree.root_node.has_error:
            entry["excluded"] = "grammar_error"
        else:
            for node in tree.root_node.named_children:
                if node.type not in kinds:
                    continue
                name = node.child_by_field_name("name")
                if name is None:
                    raise RuntimeError((rel, node.type, "missing name"))
                entry["objects"].append({
                    "kind": kinds[node.type],
                    "name": re.sub(r"\s+", "", data[name.start_byte:name.end_byte].decode("utf-8")),
                    "line": node.start_point.row + 1,
                    "column": node.start_point.column + 1,
                    "end_line": node.end_point.row + 1,
                    "start": node.start_byte, "end": node.end_byte,
                    "source": data[node.start_byte:node.end_byte].decode("utf-8").replace("\r\n", "\n"),
                })
        files.append(entry)
    implementation_root = Path(__file__).resolve().parent.parent
    source_paths = sorted(p for folder in ("internal", "cmd/funcfinder")
                          for p in (implementation_root / folder).glob("*.go"))
    report = {
        "root": str(root),
        "revision": subprocess.check_output(["git", "-C", str(root), "rev-parse", "HEAD"]).decode().strip(),
        "tree_sitter": importlib.metadata.version("tree-sitter"),
        "grammar_cli": "0.25.8 (caller-generated library; verify against upstream package.json)",
        "grammar_sha256": hashlib.sha256((root / "tree-sitter/grammar.js").read_bytes()).hexdigest(),
        "scanner_sha256": hashlib.sha256((root / "tree-sitter/src/scanner.c").read_bytes()).hexdigest(),
        "grammar_library_sha256": hashlib.sha256(library.read_bytes()).hexdigest(),
        "funcfinder_source_sha256": {p.relative_to(implementation_root).as_posix(): hashlib.sha256(p.read_bytes()).hexdigest() for p in source_paths},
        "files": files,
    }
    output = Path(args.output)
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(json.dumps(report, ensure_ascii=False, indent=2), encoding="utf-8")
    print(json.dumps({"files": len(files), "excluded": sum("excluded" in f for f in files),
                      "objects": sum(len(f["objects"]) for f in files)}))


if __name__ == "__main__":
    main()
