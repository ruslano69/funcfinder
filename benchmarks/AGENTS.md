# benchmarks/

## Purpose

Measure funcfinder on real projects using independent syntax-tree references.

## Ownership

- `specsheet.py` compares directory maps with the Go oracle and estimates token savings.
- `python_ast_eval.py` compares Python function locations and ends with Python's native AST.
- `php_tree_eval.py` compares named PHP functions/methods, including bodyless declarations, with tree-sitter PHP.
- `scala_d_tree_eval.py` compares Scala definitions/declarations, D functions/constructors/destructors/postblits and Java methods/constructors/annotation elements with their tree-sitter grammars (the historical filename is retained).
- `c_tree_eval.py` compares C definitions/prototypes after GCC preprocessing, mapped back to original source lines.
- `ts_tree_eval.py` compares TypeScript functions, methods, signatures and variable-bound functions with tree-sitter-typescript.
- `js_tree_eval.py` compares JavaScript named declarations/expressions, generators, literal methods and directly bound anonymous variable functions/arrows with tree-sitter-javascript. `JAVASCRIPT_FIX.md` records the profile, pinned corpora and before/after results.
- `cpp_tree_eval.py` compares C++ functions, constructors, destructors and bodyless declarations with libclang, keeping only the active lines of `g++ -E`.
- `csharp_roslyn_eval.py` builds `csharp_oracle.cs` against the installed SDK's Roslyn assemblies and compares C# named methods, constructors/destructors, local functions and operators, including bodyless declarations.
- `csharp_traversal_compare.py` compares production linear scope traversal with the test-only recursive prototype through a Go overlay; it records source/binary size, in-process timing/allocations, complete CLI output equality and Roslyn results for both builds.
- `brace_pilot_build.py` builds and tests the shared C#/Java/D recursive pilot through a Go overlay, recording source/binary hashes and test/vet logs. `BRACE_PILOT.md` describes the pilot, scope and independent corpus results.
- `d_traversal_compare.py` measures the full production D parser versus the verified shared recursive pilot: serial parsing/allocations on preloaded tracked D files and paired complete directory CLI runs. Raw samples and maps stay under its ignored work directory.
- `brace_parallel_compare.py` builds the optional scope-parallel overlay and compares file/scope worker combinations with verified production and serial-pilot binaries. `--language cs|java|d` selects corpus parity for all `--root` projects (default `d`); `--parity-project language=path` adds other parity corpora. `BRACE_PARALLEL.md` records the scheduler, results and limits.
- `java_finder_compare.py` compares before/after Java CLI binaries and an optional frozen pilot with one/CPU-count file workers, repeated shuffled rounds and stable per-variant output. It retains maps, counts, pins and source/binary hashes. `JAVA_FIX.md` records the production fix, independent quality checks and timing limits.
- `README.md` owns evaluator usage and measurement limitations.
- `searchserver/` contains the existing search-server load harness.

## Local Contracts

- Keep reference-project commits and the tested funcfinder commit in measurement output.
- Retain duplicate function names and report missing functions, false positives and wrong ends separately.
- Python evaluation scans tracked `.py` files and reports parse failures explicitly.
- PHP evaluation scans tracked `.php` files, reports parse failures explicitly and records parser/grammar versions. It computes line numbers from byte offsets to avoid the failing native Point wrapper on the current Windows Python 3.14 runtime.
- Scala/D evaluation excludes tree extras and trailing trivia from reference ends, reports excluded parse-error files and records grammar versions. D additionally records the downloaded grammar DLL hash and every audited grammar rejection (`oracle_rejected`): `final switch` statements and constructor-delegation calls mislabeled as declarations. Local class constructors remain in truth. Parser downloads belong in an explicitly configured build cache.
- Java truth includes methods, explicit and compact constructors, and annotation-type elements (bodyless named members). Anonymous-class methods are included; implicit record constructors and anonymous lambdas are excluded. Parse errors are explicit exclusions. Keep the Java grammar version alongside the project pin.
- C++ evaluation uses libclang as the reference (not tree-sitter: its C++ grammar cannot parse fmt's headers). It records the libclang, compiler and system include directories, lists the scored `--pathspec`, and reports unparseable files explicitly. A function template's end is read from its tokens, because libclang's extent stops before a trailing return type.
- Generated maps, clones and reports belong in ignored `build/`, not tracked source directories.
- JS truth keeps a named expression's internal name; anonymous direct identifier-variable values use the variable name. Exclude computed methods, anonymous callbacks and anonymous assignment/property values. Do not name an anonymous IIFE after the variable receiving its result. Keep duplicate names through plain-map output; match names/header lines one-to-one and compare end lines. Report grammar/runtime versions, tracked pathspec, parse-error exclusions and binary/project hashes. Read Git's UTF-8 paths explicitly on Windows. Results from this expanded profile (including vendor sources) must not be presented as a direct replay of the historical 1918-function lodash profile. JS whole-map comparisons may gain `.cjs` files; test TypeScript symbol output separately.
- C# reference parsing uses Preview syntax with `FileBasedProgram` enabled and explicit `--define` symbols (empty by default). Report parse failures and candidates in disabled text separately. Record SDK/Roslyn versions, feature flags and the compiled oracle hash. The oracle needs no project dependencies or semantic compilation; anonymous lambdas, implicit primary constructors and property/event accessors are outside named-function truth. Conversion type names use compact syntax tokens.
- C# traversal comparisons share cleaning, tokenization and recognition rules. Compare full effective finder size and traversal-only size separately; exclude test/harness code. Preload tracked files for serial Go benchmarks, repeat both traversal orders, and alternate CLI build order with warmups. Keep all samples, source/executable hashes and build flags. CLI timings include startup/I/O/output and are distinct from parser timing. Overlay input must use a non-`.go` extension so `go test ./...` does not discover it as a package.
- D traversal timing includes all tracked D files, including the six reference-grammar exclusions; report the accepted-file quality subset separately. The CLI runs `--dir --all --json --no-gitignore` on the original clones (arsd also contains one unchanged PHP source). Do not assume equivalent raw output: headers/class scopes or unscored files can differ. Require stable output within each variant, retain both maps and report output equality and counts. Verify binary and adapter/walker hashes against the pilot build before timing.
- Scope-parallel comparisons require exact ordered result and CLI-text parity against the serial pilot, including types; this does not establish parity with production or expand independent oracle coverage. Use one global extra-task budget across file workers, record the scope threshold and `GOMAXPROCS`, and shuffle measurement rounds. Windows CPU utilization is child kernel+user CPU time divided by whole-run wall time and logical CPU count: an average including startup/I/O, not peak utilization. Other platforms report wall time without CPU utilization. Retain samples, commands and hashes under ignored `build/`.
- Java before/after comparisons may change both function and type output; report quality and map counts separately instead of claiming traversal-only speed effects. Record noisy timing ranges and preserve outliers. A named-type presence/line audit does not establish exact type end boundaries.
- C measurements record the compiler version/hash and exact C99 defines/include configuration. Score active source candidates; report inactive candidates and preprocessing/parse failures separately. Mask only GCC va_arg/offsetof expressions for grammar compatibility, preserving byte/newline coordinates; function-pointer variables and typedefs are not reference functions.

## Work Guidance

- Record measured results and outstanding parser issues in root `TODO.md`.
- State reference grammar/runtime and span normalization alongside measurements.

## Verification

- Run `python benchmarks/python_ast_eval.py --help` after changing its interface.
- Run `python benchmarks/php_tree_eval.py --help` after changing its interface.
- Run `python benchmarks/scala_d_tree_eval.py --help` after changing its interface.
- Run `python benchmarks/c_tree_eval.py --help` after changing its interface.
- Run `python benchmarks/cpp_tree_eval.py --help` after changing its interface.
- Run `python benchmarks/ts_tree_eval.py --help` after changing its interface.
- Run `python benchmarks/js_tree_eval.py --help` after changing its interface, then score the pinned lodash and Express corpora before recording JavaScript coverage. Recheck TypeScript output after changes shared with its scanner/configuration.
- Run `python benchmarks/csharp_roslyn_eval.py --help` after changing its interface.
- Run `python benchmarks/csharp_traversal_compare.py --help` after changing its interface; its full run tests both builds and validates both against Roslyn on the supplied pins.
- Run `python benchmarks/brace_pilot_build.py --help` after changing its interface; the normal run builds, tests and vets both variants. Re-score the candidate with the C# and Scala/D/Java evaluators before reporting quality.
- Run `python benchmarks/d_traversal_compare.py --help` after changing its interface, then run it on the two pinned D corpora before recording performance claims.
- Run `python benchmarks/brace_parallel_compare.py --help` after changing its interface; its full run tests/vets the overlay and checks supplied corpus parity. Run `go test -race ./internal -run '^TestBraceParallel(Ordering|Shared)' -count=1` with a working C toolchain for scheduler changes.
- Run `python benchmarks/java_finder_compare.py --help` after changing its interface and perform the paired comparison before recording Java fix timings. Independently score function quality with `scala_d_tree_eval.py --language java`.
- Re-run the evaluator on the affected pinned reference projects after a measurement change.

## Child DOX Index

- `searchserver/` — existing search-server load harness.
