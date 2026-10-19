# C finder experiment in D — 2026-10-19

`c_port.d` adds a token-based C finder on top of the existing C++ tokenizer,
Unicode 15.0.0 classifiers, cleaner and result model. The standalone
`c_port_probe.d` exercises the serial finder API. Production Go and the other
D backends are unchanged; this is not yet a unified CLI.

The declarator walker follows parentheses, pointer qualifiers, array and function
suffixes. The first operator outward from the identifier distinguishes a function
from a pointer variable. It handles parenthesized names, pointer returns,
functions returning function pointers, prototype lists beginning with a function,
attributes, K&R definitions and local prototypes. Executable context prevents
`assert(check(x));` and statement macros from becoming named functions. Continued
directives and declaration-wrapper macros are skipped as balanced units.

## Independent quality

Lua is clean at `0b29f408433e92953cc72b1d3e06c7ac8139e439`:
68 tracked `.c`/`.h` files, all accepted by preprocessing and the grammar.

| Variant | Truth | Found | Missing | False | Wrong ends |
|---|---:|---:|---:|---:|---:|
| Frozen Go 1.15.9 | 1,568 | 1,568 | 0 | 0 | 0 |
| D C tokens | 1,568 | 1,568 | 0 | 0 | 0 |

Truth includes 1,177 definitions and 391 prototypes. Both variants return 1,703
raw functions; the same 135 candidates in inactive branches are retained and
reported separately. D additionally returns 68 named struct/union/enum tags;
Go returns no raw C types in this probe. Tag spans are not independently scored.
Function-name tokens must fall inside the reference header interval; final lines
must match exactly. This does not establish identical signature/start metadata.

Reference: Python 3.14.6, tree-sitter 0.26.0, tree-sitter-c 0.24.1,
MSYS2 UCRT64 GCC 15.2.0 Rev8, C99. The Lua root is an include directory,
`lua.h` is forced for standalone headers, and `LUA_USE_JUMPTABLE=0` selects
standard dispatch. GCC line markers restore original coordinates; 38
`va_arg`/`offsetof` builtin expressions are masked without changing coordinates.
Results cover this compiler configuration. D itself scans original source and
does not expand macros or select active branches.

Release and assertion maps match on all 68 corpus files and 11 fixture/stress
files. Both normal/raw cleaners match frozen Go on every file. Eight valid
fixtures independently verify 15 named declarations and their ends; the macro,
incomplete and invalid-UTF-8 fixtures have explicit expectations instead of
accepted grammar coverage. Cases include Unicode, malformed UTF-8, a 128 KiB
line, 128 nested blocks, callback variables, pointer returns, parenthesized names,
K&R headers, local prototypes and continued directives. The C module unit tests
pass separately. Assertions use `AuditBraceCleaner` and `AuditCppTokens` with
bounds checks; timed release builds contain neither audit.

## Measurements

Seven shuffled paired rounds, three preloaded passes each, with warmups:

| Mode | Go median ms | D median ms |
|---|---:|---:|
| Serial cleaner + finder | 15.456 | 17.150 |
| Complete probe process | 47.002 | 49.323 |

D takes 11.0% more parser time and 4.9% more complete-process time here. It also
walks executable scopes for local prototypes and emits tag definitions. There is
no measured C speed gain. Parser ranges are Go 15.271–15.506 ms and D
15.746–19.540 ms; the Go process has a retained 125.524 ms outlier. Complete
process timing includes startup, input and JSON generation; Python JSON decoding
is outside that timer. No compiler/oracle/audit work runs during timing.

The C-only D executable is 946,688 bytes; the generic Go probe is 2,807,296
bytes. This measures specialization as well as implementation language, not
equivalent complete-toolkit size. D uses LDC 1.43.0 with
`-O3 -release -mcpu=x86-64`; Go 1.25.7 uses `GOAMD64=v1` and stripped linking.

## Reproduce and limits

```powershell
python benchmarks/c_port_compare.py --help
python benchmarks/c_port_compare.py `
  --root build/language-eval-20261012/repos/lua `
  --work-dir build/c-d-port-20261019 --samples 7 --repeats 3
```

The harness reuses the hash-verified frozen Go binary/reference report from
`build/go-java-d-port-20261018/`; it does not build the dirty working Go tree.
It builds retained D source snapshots with the installed LDC/MSVC environment,
runs separate release/assertion/unit executables, then scores and measures.
`--samples 0` runs correctness without timing. Preserve a measured work directory
and choose a new directory when changing implementation or inputs.

`build/c-d-port-20261019/report.json` retains source snapshots/hashes, build
commands, binaries, exact input hashes, compiler/grammar settings, all findings
and raw samples. Adjacent JSONL files retain full maps and cleanup facts;
`truth.json`, `fixtures/` and `unit.txt` preserve the independent reference and
tests. Final hashes and corpus bytes are checked after timing. All nine prior
C#/C++/Go/Java D source hashes still match their previously verified snapshots;
these backends are not linked into or modified by the standalone C entrypoint.

Remaining limits: no preprocessing/macro expansion in D, no semantic resolution
of typedef-dependent ambiguities, and no complete C declarator grammar. In
particular, lists beginning with an ordinary variable and initialized list items
need further handling. Inside executable scopes a bare `word(args);` is treated
as a call; an unusually parenthesized prototype using only a typedef name as
its prefix can therefore be ambiguous. Recursive declarators stop at depth 256.
Tag results need an independent type oracle. The complete CLI, file workers,
selection/extraction and other toolkit analyses remain outside this probe.

DOX: benchmark ownership/contracts and README plus root TODO are updated. Root
and other child AGENTS/indexes stay unchanged because production ownership,
CLI contracts and repository boundaries are unchanged.
