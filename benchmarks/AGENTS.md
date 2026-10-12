# benchmarks/

## Purpose

Measure funcfinder on real projects using independent syntax-tree references.

## Ownership

- `specsheet.py` compares directory maps with the Go oracle and estimates token savings.
- `python_ast_eval.py` compares Python function locations and ends with Python's native AST.
- `README.md` owns evaluator usage and measurement limitations.
- `searchserver/` contains the existing search-server load harness.

## Local Contracts

- Keep reference-project commits and the tested funcfinder commit in measurement output.
- Retain duplicate function names and report missing functions, false positives and wrong ends separately.
- Python evaluation scans tracked `.py` files and reports parse failures explicitly.
- Generated maps, clones and reports belong in ignored `build/`, not tracked source directories.

## Work Guidance

- Record measured results and outstanding parser issues in root `TODO.md`.
- State reference grammar/runtime and span normalization alongside measurements.

## Verification

- Run `python benchmarks/python_ast_eval.py --help` after changing its interface.
- Re-run the evaluator on the affected pinned reference projects after a measurement change.

## Child DOX Index

- `searchserver/` — existing search-server load harness.
