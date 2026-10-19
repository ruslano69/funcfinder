# Scala finder API in D

Measured on Windows x64, 2026-10-19. The dedicated lexer/finder discovers named
`def` functions/signatures, including parameterless/operator/auxiliary-constructor
names, and named class/object/package-object/trait/enum/type declarations. Finder
prototypes now cover 13 of 15 languages. Python/Ruby and the shared CLI remain open.

## Independent correctness

Reference: Python 3.14.6, tree-sitter 0.26.0, tree-sitter-scala 0.26.2. Scan all
tracked `.scala` files, including platform variants/tests. The syntax profile uses
`function_definition`/`function_declaration` and named `class_definition`,
`object_definition`, `package_object`, `trait_definition`, `enum_definition` and
`type_definition`. Backtick names are unquoted. Lambdas, anonymous templates,
enum cases and literal/interpolation/XML/quote contents are outside named truth.
Extension blocks supply their named methods, not a duplicate type. Type aliases,
abstract types, package objects and nested/local types are independently scored.

Name/header anchors match one-to-one, retaining duplicates; final significant
syntax tokens determine exact ends. Coordinates use UTF-8 byte offsets, with CRLF
normalized in reference memory. Error auditing visits all children, including
unnamed missing tokens. Every corpus file parses and has an explicit input hash.

| Project | Pin | Files | Functions | Named types | D missing/false/wrong ends |
|---|---|---:|---:|---:|---|
| scopt | `fb371a7cf997fe053c9e1496cc6e13fbff3ff9de` | 29 | 380 | 90 | 0/0/0, both profiles |
| os-lib | `86e5ddb65fe62305afb5885fcc0f8da50aba4b48` | 59 | 643 | 238 | 0/0/0, both profiles |

Both checkouts are clean. Frozen Go from
`bdbc81bdc9be260782163d62c49e8b6135a0923d` (1.15.9), verified through the preceding
PHP report, also has zero function errors. Its narrower raw type output is 46/147
against D's 90/238 (44/91 reference misses, zero false types/wrong ends). This
broader discovery is not a direct replay of the historical Go type profile or
ordered Go-map/cleanup parity. The dirty Go working tree is not a measurement input.

## Lexer, boundaries and checks

`scala_port.d` retains immutable token slices, byte/line/column metadata and paired
delimiters. It owns nested block comments, normal/triple strings and interpolator
prefixes, nested interpolation, backticks, symbolic/operator-suffix identifiers
and character/symbol-literal disambiguation. ASCII/Go Unicode 15 Letter/Digit plus
combining marks cover tested identifiers; this is not full Scala Unicode grammar.

The finder handles type/parameter clauses, curried/context parameters, bodyless
declarations, procedures, expression/control bodies, infix/member continuations,
refinement return types and nested functions. Expressions stop before the next
annotation/declaration. Limited Scala 3 indentation support retains nested body
statements and explicit `end` markers; constructor/header braces are paired once.
Owner metadata is lexical and is not independently semantic truth.

Separate release, bounds/assertion (`AuditScalaTokens`) and unit builds pass. All
88 corpus maps match between release and audit. Assertions check source slices,
byte ordering, exact first/final lines, initial byte columns and pair symmetry.
Twenty-two fixtures cover modifiers/annotations, parameterless/backtick/operator/
setter names, generics/curried/context clauses, auxiliary constructors, procedures,
abstract/alias types, package objects, expression/control bodies, nested
interpolation and symbol/character literals, Scala 3 templates/enums/extensions/end
markers, Unicode/CRLF, a 128 KiB line, 128 nested blocks and 64 nested functions.
Twenty valid fixtures independently verify 107 functions and 15 types with no
errors. Incomplete and invalid-UTF-8 inputs are explicit unscored robustness checks.
Malformed input normalizes one bad byte at a time before tokenization.

## Timing and size

LDC 1.43.0, `-O3 -release -mcpu=x86-64`; Go 1.25.7, `GOAMD64=v1`. Seven shuffled
paired rounds with three preloaded serial parses per parser sample and excluded
warmup. Oracle/audit/build work is outside timing. Process samples include
startup/read/parse/JSON and retain stable per-variant maps. All 88 files are timed,
all raw samples retained. Median milliseconds:

| Project | Go parser | D parser | Parser ratio | Go process | D process | Process ratio |
|---|---:|---:|---:|---:|---:|---:|
| scopt | 8.634 | 3.309 | 2.61x | 37.797 | 32.977 | 1.15x |
| os-lib | 23.835 | 5.863 | 4.07x | 55.829 | 36.913 | 1.51x |

Scala-only D release: 940,032 bytes; generic Go: 2,807,296 bytes. Dedicated parsing
algorithms, type profiles and executable specialization differ. These are not
language-only speed/size comparisons or equivalent complete toolkit executables.
Startup limits whole-process gains on these small corpora.

## Reproduction, evidence and limits

```powershell
python benchmarks/scala_port_compare.py --help
python benchmarks/scala_port_compare.py `
  --project scopt=build/language-eval-20261012/repos/scopt `
  --project oslib=build/language-eval-20261012/repos/os-lib `
  --work-dir build/scala-d-port-final-20261019 --samples 7 --repeats 3
```

Pinned grammar/runtime packages live in `build/eval-runtime/`. LDC/MSVC uses
`F:\D\ldcvars64.bat`; the frozen Go source chain comes through
`build/php-d-port-final-20261019/report.json`. Nineteen earlier D backend hashes
are unchanged. `--samples 0` runs correctness only; choose a new work directory
for changed sources/inputs to retain earlier measurements.

The work directory retains source/helper snapshots, compiler commands/unit logs,
configs/manifests, Go/D/audit maps, independent truth/diagnostics, fixture inputs,
binary/compiler/grammar/input hashes and raw samples. Final hashes are checked
after timing. `postflight.py`/`postflight.json` independently recheck truth/maps,
hashes and unsupported configurations. There is no Scala compiler/semantic build.

The measured projects predominantly use Scala 2 syntax. Scala 3 coverage comes
from the listed fixtures; complete indentation/soft-keyword, quote/macro/XML and
Unicode/operator grammar remain unverified. Interpolation depth stops at 256;
columns are byte columns, not a generalized tab-layout engine. Common CLI
selection/extraction, directory workers and other toolkit analyses remain open.
Production Go and previous backends are unchanged.

DOX: benchmark ownership/contracts, README and root TODO are updated. Root and
other child AGENTS/indexes remain unchanged because repository boundaries,
production ownership and shared CLI contracts did not change.
