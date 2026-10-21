# Experimental funcstats in D

The second D tool shares `ffd`'s 15 native finders, language configuration, reusable tokenizers and directory walker.
Production Go tools and release archives are unchanged. Build with:

```powershell
python benchmarks/ffd/build.py --tool funcstats --test
python benchmarks/ffd/build.py --tool funcstats
python benchmarks/ffd/funcstats_checks.py --d build/ffd/funcstats.exe --go funcstats.exe
```

The checker requires Go on PATH; it compiles `funcstats_go_ast.go.txt` under ignored `build/`.

## CLI and output

All advertised Go options are implemented: positional source file, `--dir`, `-l`, `-n`, `-j`/`--json`,
`--uncalled`, repeatable `--exclude`, `--version`, `-h`/`--help`. Flags can follow the filename.
Language auto-detection, dominant directory language/ties, root ignore/hidden rules, sorted depth-first file order,
positive top-N limits and alphabetical count ties retain the common Go contract. Forced C includes `.h` files.
Directory jobs use eight threads, write separate slots and aggregate in walk order. Empty lists encode as arrays.
JSON field names/values retain the Go schema, while whitespace/key order/slash escaping can differ. Controlled
text output is byte-compared with Go, including per-file/directory summaries and uncalled text.

D explicitly fails on unknown options, missing option values and malformed `-n`; Go silently ignores/coerces
some such input. These are deliberate validation differences, not missing flags. The default Go command's permissive
parser is not a target for silently accepting typos.

## Counting and scope

- Calls are syntactic named invocations, not symbol-resolved targets. Identifier conversions such as `int(x)` count;
  composite-type conversions such as `[]byte(nil)` do not. Go control keywords and interface signatures are not calls.
  Generic/indexed Go callees are included. Other complex/indirect/operator/cast syntax remains profile-dependent.
- `ffd_calls.d` shares declaration exclusion and call recognition with the D callgraph. JS private method names
  count as both calls and code uses; optional receivers retain their terminal call names. The field/discovery token
  adapter is unchanged. The extra private-method regression is independent of Go parity.
- Function bounds and owned signature metadata come from the existing finder. Only the declaration occurrence is
  suppressed; recursive calls on that line remain. JS arrow/anonymous bindings retain their real recursive calls.
  Signature metadata is requested through `ScanNeeds` and copied only for call-site consumers; ordinary `ffd`
  discovery retains its previous symbol size and avoids these copies.
  The statistics consumer applies lexical call-site context checks, without a second declaration finder.
- `--uncalled` is a conservative identifier-use analysis: callback references and recursion count; comments and
  strings do not. Test functions, main/init and the documented Go implicit methods are excluded. Tests can still
  use production names. Exported Go candidates outside `internal` are marked. No reachability, semantic owner,
  dynamic dispatch, reflection or cross-repository resolution is claimed.
- Lines partition into code, comments and blanks using the existing native lexical profiles. Blank physical lines
  stay blank; there is no phantom line after final LF. Native tokenizers retain some literals and omit others,
  so literal-only/multiline literal classifications can differ by backend and from Go. `comment_lines` can include
  nonblank lines left without code by a cleaner; it is not a full comment-token measurement.
- Import/annotation extraction retains configured patterns after an active-lexer-line gate. This rejects raw matches
  inside comments/literals but is not a complete import/dependency grammar. In particular grouped/aliased/dynamic
  syntax remains limited by the configured profile. Go grouped quoted imports retain their existing profile.
- Existing finder accuracy/analysis-scope differences remain visible. Fifteen-language fixture support does not
  establish full grammar accuracy. Parser and token passes still run separately. CLI use-map analysis now runs
  only for `--uncalled`; returned maps retain owned distinct names. See [buffer measurements](STATS_BUFFERS.md)
  for scratch reuse, independent Go count checks and measured corpus-specific gains.

## Verification on 2026-10-21

189 funcstats CLI/truth checks pass after callgraph integration; its unit build passes 22 modules. The initial port's shared-module changes also passed
the 30-module funcfinder unit build and 3,137 existing funcfinder CLI checks.

The CLI checker covers controlled 15-language call truth and text/JSON parity, empty/CRLF files, imports,
annotations, recursion/split headers/JS bindings, top-N, dominant/forced languages, directory order and ignores,
repeated/path exclusions, uncalled test/internal/callback rules, malformed flags, Unicode, invalid UTF-8 and a
128 KiB line. Independent Python AST and Go AST fixtures check call truth; imports in comments/literals and
decorated uncalled declarations have explicit source-truth checks rather than copying Go's false results.

A separate local-project Go AST audit uses Go 1.27.1 `go/parser`/`go/ast`. It extracts named `CallExpr` identifiers,
selectors and generic/indexed callees, applies only the configured exclusions and never counts declarations.
It does not use `go/types`, so identifier type conversions remain part of the named-call syntax profile.

| Input | Files | Physical lines | D differing file/name pairs | Go differing file/name pairs |
|---|---:|---:|---:|---:|
| `cmd/funcstats` | 5 | 850 | 0 | 3 |
| `internal` | 137 | 27,687 | 0 | 61 |

Go has 108 excess call occurrences across these 64 pairs and no missing occurrences in this profile. They are
control words, composite-type conversions and interface signatures. D removes these false calls; equality with
Go is therefore intentionally not the accuracy criterion. This audit covers these local Go files only.

The source, binaries, AST maps/count deltas and input/compiler hashes are retained under ignored
`build/funcstats-validation/`. Temporary fixture trees are removed by the checker. No test repositories were cloned.
