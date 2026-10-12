# benchmarks/

## Purpose

Measure funcfinder on real projects using independent syntax-tree references.

## Ownership

- `specsheet.py` compares directory maps with the Go oracle and estimates token savings.
- `python_ast_eval.py` compares Python function locations and ends with Python's native AST.
- `php_tree_eval.py` compares named PHP functions/methods, including bodyless declarations, with tree-sitter PHP.
- `scala_d_tree_eval.py` compares Scala definitions/declarations and D functions/constructors/destructors/postblits with their tree-sitter grammars.
- `README.md` owns evaluator usage and measurement limitations.
- `searchserver/` contains the existing search-server load harness.

## Local Contracts

- Keep reference-project commits and the tested funcfinder commit in measurement output.
- Retain duplicate function names and report missing functions, false positives and wrong ends separately.
- Python evaluation scans tracked `.py` files and reports parse failures explicitly.
- PHP evaluation scans tracked `.php` files, reports parse failures explicitly and records parser/grammar versions. It computes line numbers from byte offsets to avoid the failing native Point wrapper on the current Windows Python 3.14 runtime.
- Scala/D evaluation excludes tree extras and trailing trivia from reference ends, reports excluded parse-error files and records grammar versions. D additionally records the downloaded grammar DLL hash and every audited grammar rejection (`oracle_rejected`): `final switch` statements and constructor-delegation calls mislabeled as declarations. Local class constructors remain in truth. Parser downloads belong in an explicitly configured build cache.
- Generated maps, clones and reports belong in ignored `build/`, not tracked source directories.

## Work Guidance

- Record measured results and outstanding parser issues in root `TODO.md`.
- State reference grammar/runtime and span normalization alongside measurements.

## Verification

- Run `python benchmarks/python_ast_eval.py --help` after changing its interface.
- Run `python benchmarks/php_tree_eval.py --help` after changing its interface.
- Run `python benchmarks/scala_d_tree_eval.py --help` after changing its interface.
- Re-run the evaluator on the affected pinned reference projects after a measurement change.

## Child DOX Index

- `searchserver/` — existing search-server load harness.
