# Ruby finder API in D

Measured on Windows x64, 2026-10-19. The standalone lexer/token/block finder
discovers named methods/singleton methods and named class/module spans. With the
Python prototype, all 15 supported languages now have D finder experiments;
this does not implement a combined CLI or the other four toolkit analyses.

## Independent correctness

Reference: Python 3.14.6, tree-sitter 0.26.0, tree-sitter-ruby 0.23.1. Scan all
tracked `.rb` files, including tests/vendor sources. Function truth is `method`/
`singleton_method`; type truth is named `class`/`module`. Include nested/local
methods, operator/setter names and explicit instance/global/singleton receivers.
Keep qualified and absolute type names. Anonymous singleton classes supply
scopes rather than duplicate named types. Alias/dynamic definitions and literal/
interpolation contents are excluded. Names/header anchors match one-to-one with
duplicates retained; end lines use final syntax tokens rather than tree extras.

Ruby's grammar represents heredoc bodies separately from their opening syntax.
Pair sorted heredoc beginnings/bodies, require equal counts, and extend a named
declaration's end through any corresponding physical terminator beyond its syntax
node. This matters for endless `def f = <<~TAG` methods. The normalization is
retained in the oracle source; it is not a native Ruby semantic/compilation check.
CRLF normalizes in reference memory. Every ERROR/missing token is audited through
all children. None of the 1,294 corpus files failed parsing.

| Project | Clean pin | Files | Functions | Named types | D missing/false/wrong ends |
|---|---|---:|---:|---:|---|
| Huginn | `74581b082883cd9aa894db7c5671fa7fda6c579f` | 500 | 1,862 | 533 | 0/0/0, both profiles |
| Maybe | `77b5469832758d1cbee1a940f3012a1ae1c74cd3` | 794 | 2,284 | 782 | 0/0/0, both profiles |

Frozen Go is the generic finder API at
`bdbc81bdc9be260782163d62c49e8b6135a0923d` (1.15.9), the production factory's
Ruby path. It is hash-verified through the Python report's retained `go_generic`
binary, not the Python-specific adapter. Huginn Go function missing/false/wrong
ends are 20/1/0; Maybe is 0/0/3. D closes these accepted-profile gaps without
losing a previously correct named method. These fresh pins/profiles are not a
direct replay of the older 1,865-method Huginn measurement. Production Go's Ruby
TODO remains open; no production parser changes are made here.

Raw Go/D type counts are 492/533 and 779/782. The older Go type profile has
missing/false/wrong-end counts 55/14/405 and 252/249/216 against this named syntax
profile. Raw types/scopes can represent different constructs; do not interpret
these as equivalent type maps, cleanup parity or semantic-owner validation.

## Lexer, block boundaries and checks

`ruby_port.d` retains immutable UTF-8 slices and pairs delimiters once. It owns
quoted/backtick/percent/regex strings, nested interpolation, escapes, regex
character classes, variables/symbols/character literals, heredoc queues and
indented labels, line/block comments and `__END__` data sections. A slash following
an operand needs a closing delimiter on its line before being treated as a
command-argument regex; this preserves tested divisions such as `x /y`.

Method discovery separates names, receivers and endless-body assignment from
setter/default-parameter assignments. A keyword stack distinguishes prefix value
conditions after assignment/shift operators from postfix modifiers after values
and variables. Optional loop `do` does not create a second block; ordinary call
blocks do. Named methods inside methods/blocks remain discoverable. Absolute/
qualified classes and anonymous singleton-class scopes are retained separately.

Separate module units and `AuditRubyTokens` bounds/assertion builds pass. All
1,294 corpus maps equal their assertion maps. Audits verify immutable slices,
byte ordering, exact first/final lines and pair symmetry. Twenty-two fixtures
cover nested/endless/operator/setter/singleton methods, defaults and absolute
types, conditional values/modifiers, loop/call/singleton blocks, quoted/percent/
regex/interpolated literals, division, queued quoted/indented heredocs and endless
heredoc spans, block comments/data sections, Unicode/CRLF, a 128 KiB line, 128
nested blocks and 64 nested methods. Twenty valid fixtures independently verify
94 methods and six types without errors; incomplete and invalid-UTF-8 inputs are
explicit unscored robustness checks. All 23 prior D backend hashes are unchanged.

## Timing and size

LDC 1.43.0, `-O3 -release -mcpu=x86-64`; Go 1.25.7, `GOAMD64=v1`. Seven shuffled
paired rounds with three serial preloaded parses per parser sample and excluded
warmup. Oracle/build/audit work is outside timing. Complete process samples
include startup/read/parse/JSON, checking stable per-variant output. Every input
and every timing sample is retained. Median milliseconds:

| Project | Go parser | D parser | Parser ratio | Go process | D process | Process ratio |
|---|---:|---:|---:|---:|---:|---:|
| Huginn | 57.299 | 34.772 | 1.65x | 107.463 | 79.471 | 1.35x |
| Maybe | 42.503 | 27.921 | 1.52x | 100.600 | 81.006 | 1.24x |

Maybe D parser samples range 27.549–33.381 ms; no samples are removed. Ruby-only
D release is 944,640 bytes; generic Go is 2,807,296 bytes. Different algorithms,
discovery profiles and executable specialization prevent language-only speed/size
claims. The probe has no directory workers, CLI selection/extraction or the other
toolkit analyses; the 15 finder prototypes are not one equivalent executable.

## Reproduction, retained evidence and limits

```powershell
python benchmarks/ruby_port_compare.py --help
python benchmarks/ruby_port_compare.py `
  --project huginn=build/ruby-d-port-20261019/repos/huginn `
  --project maybe=build/ruby-d-port-20261019/repos/maybe `
  --work-dir build/ruby-d-port-final-20261019 --samples 7 --repeats 3
```

Pinned Python grammar/runtime packages live under `build/ruby-eval-runtime/`.
LDC/MSVC uses `F:\D\ldcvars64.bat`. The frozen source/hash chain is
`build/python-d-port-production-20261019/report.json`. Use a new work directory
for changed sources/inputs and `--samples 0` for correctness only. No Ruby runtime
or application dependencies are needed for the syntax reference.

The work directory retains source/helper snapshots, compiler commands, unit/build
logs, configs/manifests, full Go/D/audit maps, independent truth/diagnostics,
fixture inputs, source/compiler/grammar/binary/input hashes and all raw samples.
Final hashes are checked after timing. `postflight.py`/`postflight.json` reparse
truth, rerun maps, verify all source/input hashes and reject eight invalid configs.

Limits: contextual regex/percent/heredoc and modifier decisions cover these pins/
fixtures, not full Ruby lexical/semantic grammar. Full Unicode identifier and
exotic character/receiver/literal forms remain unverified; interpolation stops at
256. Literal/interpolation declarations and dynamic/alias methods are outside
this named syntax profile. Owners are lexical, not independently semantic. DOX
benchmark ownership/contracts, README and root TODO are updated; root and other
child AGENTS/indexes remain unchanged because repository boundaries, production
ownership and common CLI contracts did not change.
