# PHP finder API in D

Measured on Windows x64, 2026-10-19. The standalone token finder supplies named
functions/methods and named class/interface/trait/enum spans. This brings finder
prototypes to 12 of 15 languages; it is not a combined CLI or the other four tools.

## Independent correctness

Reference: Python 3.14.6, tree-sitter 0.26.0, tree-sitter-php 0.25.1. All tracked
`.php` files, including tests/fixtures, are included. Functions use
`function_definition`/`method_declaration`, including bodyless members and methods
of anonymous/local classes. Anonymous closures and anonymous types are not named
truth. Types use named class/interface/trait/enum declarations. Each name/header
anchor matches one-to-one; final significant syntax tokens define exact end lines.
Coordinates come from UTF-8 byte offsets, with CRLF normalized in reference memory.
Every ERROR/missing token is audited through all children. No corpus files failed.

| Project | Pin | Files | Functions | Named types | D missing/false/wrong ends |
|---|---|---:|---:|---:|---|
| Monolog | `3bed304b1905fed5c30d172872f4d890cbb03c6a` | 228 | 1,602 | 261 | 0/0/0, both profiles |
| Symfony Routing | `57970507cf869e358330cb336772a7dcb31fa67c` | 267 | 1,125 | 219 | 0/0/0, both profiles |

Both checkouts are clean. The frozen Go probe is from
`bdbc81bdc9be260782163d62c49e8b6135a0923d` (1.15.9), verified through the preceding
Swift report; the dirty Go working tree is never compiled for these measurements.
Go also has zero function errors on these projects. Its older type map misses
2/6 types and has 0/1 false types respectively; D's expanded type profile resolves
these differences. Raw Go/D type counts are 259/261 and 214/219. These results do
not establish ordered Go-map/cleanup parity or semantic-owner accuracy.

## Implementation and checks

`php_port.d` scans immutable UTF-8 slices and pairs delimiters once. Keywords
compare ASCII case-insensitively; PHP identifiers admit high bytes independently
of Unicode categories. The lexer owns PHP/HTML transitions, short/echo tags,
non-nested block comments, `//`/`#` comments, attributes, quoted/backtick strings,
interpolation, heredoc/nowdoc and indented closing labels. Variables cannot become
keywords. Strings, interpolation and HTML do not supply function candidates.
Unquoted heredoc labels and anonymous `class extends/implements` have regression
fixtures. Anonymous classes supply empty lexical owner scopes without named types.

Separate release, `AuditPHPTokens` bounds/assertion and unit executables pass.
The audit checks immutable slices, byte bounds, exact first/final line coordinates,
ordering and pair symmetry. All 495 corpus maps match between release and audit.
Twenty-one fixtures cover attributes, references, DNF return types, constructors
and promoted fields, closures, case-insensitive keywords, anonymous/local classes,
PHP/HTML/comment transitions, string interpolation, quoted/unquoted/indented
heredoc labels, CRLF/Unicode, a 128 KiB line, 128 nested blocks and 64 nested named
functions. Nineteen valid fixtures independently verify 94 functions and eight
types with zero errors; incomplete and invalid-UTF-8 inputs are explicit robustness
checks outside independent truth. Malformed input is normalized one bad byte at
a time before tokenization. The module unit suite passes separately.

## Timing and size

LDC 1.43.0 (`-O3 -release -mcpu=x86-64`), Go 1.25.7 (`GOAMD64=v1`). Seven shuffled
paired rounds, three preloaded serial parses per parser sample, excluded warmup;
complete process samples include startup/read/parse/JSON and retain each variant's
own stable output. Oracle/build/audit work is outside timing. All 495 inputs are
timed. Values are median milliseconds.

| Project | Go parser | D parser | Parser ratio | Go process | D process | Process ratio |
|---|---:|---:|---:|---:|---:|---:|
| Monolog | 38.611 | 12.632 | 3.06x | 77.852 | 50.419 | 1.54x |
| Symfony Routing | 44.612 | 13.850 | 3.22x | 86.153 | 52.694 | 1.63x |

The Routing Go process has a retained 166.730 ms outlier; D's maximum is 69.298 ms.
No samples are removed. D's PHP-only release is 929,792 bytes; generic Go is
2,807,296 bytes. Algorithms, type profiles and specialization differ, so neither
speed nor size is a language-only comparison or an equivalent toolkit benchmark.

## Reproduction and retained evidence

```powershell
python benchmarks/php_port_compare.py --help
python benchmarks/php_port_compare.py `
  --project monolog=build/language-eval-20261012/repos/monolog `
  --project routing=build/language-eval-20261012/repos/symfony-routing `
  --work-dir build/php-d-port-final-20261019 --samples 7 --repeats 3
```

Install the pinned Python grammar/runtime under `build/eval-runtime/`. LDC/MSVC
uses `F:\D\ldcvars64.bat`. The previous report is
`build/swift-d-port-20261019/report.json`; its frozen Go hash and 17 earlier D
backend source hashes are checked. `--samples 0` runs correctness only. Use a new
work directory for changed sources/inputs to retain earlier measurements.

The work directory retains source/helper snapshots, configs/manifests, builds,
binary/compiler/grammar/input hashes, full Go/D/audit maps, independent truth,
fixture/unit logs and all timing samples. Final hashes are checked after timing.
`postflight.py`/`postflight.json` independently recheck retained truth/maps/hashes
and reject unsupported configurations. No PHP runtime/semantic build is required.

Limits: interpolation depth is capped at 256; the lexer is not a complete PHP
grammar/semantic validator. PHP extensions, complex interpolation and future
syntax beyond these corpora/fixtures remain unverified. Short tags are recognized
unconditionally. Common CLI selection/extraction, file workers and other toolkit
analyses remain open. Production Go and the previous backends are unchanged.

DOX: benchmark ownership/contracts, README and root TODO are updated. Root and
other child AGENTS/indexes remain unchanged: repository boundaries, production
ownership and shared CLI contracts are unchanged.
