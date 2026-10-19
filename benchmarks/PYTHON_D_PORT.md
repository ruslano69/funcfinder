# Python finder API in D

Measured on Windows x64, 2026-10-19. The standalone D lexer/logical-statement/
indentation finder supplies named `def`/`async def` and class spans, including
nested/local declarations. Lambdas and type aliases are outside this profile.

## Native correctness and Go baseline

Reference: CPython 3.14.6 native `ast.FunctionDef`, `ast.AsyncFunctionDef` and
`ast.ClassDef`, with duplicate names retained. Match each declaration one-to-one
between its first decorator and definition line; D ends must exactly equal native
`end_lineno`, with no D end trimming. Scan all tracked `.py` files. Syntax/encoding
errors are explicit exclusions; none occurred in these 180 corpus inputs.

| Project | Clean pin | Files | Functions | Classes | D missing/false/wrong ends |
|---|---|---:|---:|---:|---|
| Flask | `d086db856be187255b8ec61ef409357393020f32` | 83 | 1,462 | 160 | 0/0/0, both profiles |
| Requests | `611c6162cbc4ac2020a2f91c7cfa4f3abf9bbb60` | 37 | 711 | 96 | 0/0/0, both profiles |
| hr_portal | `87a2909d85ee173209afcecdd12f530257e5d2ab` | 60 | 711 | 87 | 0/0/0, both profiles |

The Go baseline is frozen at `bdbc81bdc9be260782163d62c49e8b6135a0923d`
(1.15.9). Production `CreateFinder` selects `PythonFinder` for indentation-based
languages. The older general-purpose Go benchmark probe calls `NewFinder`, so it
is not the Python production baseline. Final measurements use an isolated Git
archive and a Go overlay appending a preloaded-lines adapter to `PythonFinder`.
The complete original method remains; the adapter copies its exact body after
file reading. Input preloading uses the original `os.ReadFile`/`strings.Split`
behavior, including CR bytes and the final empty line. Source/overlay/probe hashes
and the exact adapter body/reader are independently checked after timing.

Retain Go's raw full maps separately from native significant-end evaluation.
`postflight.json` also trims only trailing blank/comment lines, as the historical
Python evaluator does. After this Go-only normalization, function missing/false/
wrong-end counts are Flask 0/0/0, Requests 29/0/1 and hr_portal 0/0/0. Requests'
raw Go map contains 682 functions; D finds all 711. All 343 classes are present
in both maps. These frozen-API results are not a replay of an earlier local Go
build or a current-release regression diagnosis. The dirty production sources
are not built or modified for this experiment.

## Lexer, suites and checks

`python_port.d` tokenizes immutable source slices and pairs delimiters once.
Logical newlines ignore brackets, escaped newlines and string contents. Statement
indentation expands tabs to eight-column stops; a monotonic stack assigns suite
ends without walking every enclosing function body again. Definitions can use
multiline parameters/annotations/type parameters, decorators, inline suites and
arbitrarily nested class/function/control statements. Ends use the final lexical
statement rather than trailing comments/blank lines.

The lexer owns normal/raw/byte/triple/f/t strings, nested replacement expressions,
same-quote nesting and replacement comments. Universal newlines normalize in
memory; UTF-8 BOM is skipped. Malformed UTF-8 normalizes one bad byte at a time for
robustness, independently of native-AST acceptance. Names retain input spelling;
complete Python XID/NFKC identifier normalization is not implemented.

Separate module unit and `AuditPythonTokens` bounds/assertion builds pass. All
180 corpus maps equal their assertion-build maps. Fifteen fixtures cover nested/
async/generic/decorated/inline suites, implicit/explicit continuations, all control
branches, tabs/CRLF/CR/BOM, docstrings/raw/f/t strings and nested quotes/comments,
Unicode, a 128 KiB line and 64 nested named functions. Thirteen valid fixtures
independently verify 87 functions and three classes with zero errors. Incomplete
and invalid-UTF-8 fixtures are explicit unscored robustness inputs. Token audits
check slices, ordering, exact first/final lines and pair symmetry. All 21 earlier
D backend source hashes remain unchanged.

## Timing and size

LDC 1.43.0, `-O3 -release -mcpu=x86-64`; Go 1.25.7, `GOAMD64=v1`. Seven shuffled
paired rounds, three serial parses over preloaded input per sample, excluded
warmup. AST/build/assertion work is outside timing. Full process samples include
startup/read/parse/JSON and verify each variant's stable maps. All 180 inputs and
all timing samples are retained. Median milliseconds:

| Project | Go parser | D parser | Parser ratio | Go process | D process |
|---|---:|---:|---:|---:|---:|
| Flask | 15.672 | 9.815 | 1.60x | 48.636 | 43.797 |
| Requests | 10.242 | 6.744 | 1.52x | 41.048 | 37.810 |
| hr_portal | 21.736 | 12.973 | 1.68x | 55.020 | 46.026 |

Python-only D release is 932,352 bytes; the production-PythonFinder Go probe is
2,555,392 bytes. Different algorithms/result details and standalone executable
specialization prevent language-only speed/size claims. Requests produces more
functions in D; the comparison is not ordered whole-map parity. These are serial
finder APIs, not an equivalent full toolkit or file-parallel CLI.

## Reproduction and retained evidence

```powershell
python benchmarks/python_port_compare.py --help
python benchmarks/python_port_compare.py `
  --project flask=build/language-eval-20261012/repos/flask `
  --project requests=build/language-eval-20261012/repos/requests `
  --project hr=G:/DEV/Python/hr_portal `
  --work-dir build/python-d-port-production-20261019 --samples 7 --repeats 3
```

Use CPython 3.14.6, installed Go and `F:\D\ldcvars64.bat` for LDC/MSVC. The previous
source/hash chain is `build/scala-d-port-final-20261019/report.json`. Choose a new
work directory for changed sources/inputs; `--samples 0` skips timing. No project
dependencies, application execution or external Python grammar are required.

The directory retains the frozen Go archive/overlay/probe, D/helper snapshots,
commands/configs/manifests, unit/build logs, full Go/D/audit maps, native truth,
fixture inputs, source/binary/interpreter/AST-library/input hashes and raw samples.
`postflight.py`/`postflight.json` reparse truth, rerun maps, verify the adapter,
snapshots and all hashes, reject eight invalid configurations and record separate
Go end normalization. Earlier exploratory generic-Go/ReadLines comparisons under
`build/python-d-port-20261019/`, `python-d-port-final-20261019/` and
`python-d-port-cli-20261019/` are not final performance evidence.

Remaining limits: complete Unicode XID/NFKC handling, legacy source encodings and
future/uncovered Python grammar; interpolation nesting stops at 256. Owners are
lexical, not independently semantic. Shared CLI selection/extraction, file workers
and other toolkit analyses remain open. DOX benchmark ownership/contracts, README
and root TODO are updated; root/other child AGENTS and indexes remain unchanged
because repository boundaries, production ownership and common CLI did not change.
