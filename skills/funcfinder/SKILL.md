---
name: funcfinder
description: Map repositories, locate and extract functions/types, trace calls and dependencies, and edit Go/Rust/C#/D/JS/TS/Kotlin/Goose declarations through versioned JSON code objects. Use for code navigation, impact analysis and function/type replacement with the funcfinder toolkit.
---

# funcfinder

Use the toolkit to locate declarations before reading source. Treat a function or type as the editing unit;
reason about its contract and replacement code, leaving byte selection and stale-file checks to the tool.
Skill use does not authorize changing unrelated repositories, publishing, or messaging others.

## Locate the current binaries

The five tools are `funcfinder`, `funcstats`, `deps`, `callgraph`, `complexity` (add `.exe` on Windows).
Check `funcfinder --help` for `--read-object` and `--edit-object` before choosing the JSON workflow.
Do not infer capability from the version number alone or use an older learning snapshot.

Prefer the binary from the active maintained checkout. On this user's Windows machine that checkout is
`H:\Ruslan\Code\Go\funcfinder` (`funcfinder.exe` there after `build.ps1`). Verify it with `funcfinder --version`
before using it from another repository. On Unix build with `build.sh`. Do not run a funcfinder build inside the
target application's repository. Check `funcfinder --help` for `--read-object` / `--edit-object`; if they are
missing, the binary predates JSON editing (v1.16.0+) — rebuild or update rather than falling back to legacy edits.

## Python intermediary (for agents that drive commands through Python)

Agents that cannot run shell pipelines safely should call funcfinder through
`scripts/funcfinder_edit.py` (stdlib only, UTF-8 pipes, works on Windows). It never interpolates source into a
command line; JSON goes through stdin.

```bash
python scripts/funcfinder_edit.py read file.go go Count > object.json   # --kind type|property|accessor, --line/--column
# change ONLY object.json's "source" field (use a JSON-aware editor, not string substitution in a shell)
python scripts/funcfinder_edit.py preview object.json                   # validates, writes nothing
python scripts/funcfinder_edit.py apply object.json                     # the SAME file, after reviewing preview
```

Exit code 1 means funcfinder refused the request; stdout then holds the structured `error.code` envelope.
Branch on `error.code` (table in [references/edit.md](references/edit.md)), never on the message text.
Set `FUNCFINDER` to the binary path when it is not on PATH. From Python, import the helpers instead:

```python
import sys; sys.path.insert(0, "scripts")
import funcfinder_edit as ff
obj = ff.read_object("file.go", "go", "Count")
obj["source"] = "func Count() int { return 42 }"
ff.preview(obj)              # inspect the result before applying
applied = ff.apply(obj)      # same request, unchanged
```

Never run `apply` with a request you have not previewed, and never rebuild a revision by hand after a
`stale_revision` refusal: read the object again and reconcile the change.

## Orient and select

Honor applicable repository instructions. For a small project:

```bash
funcfinder --dir . --all --json
```

For 50+ files, create/load a split index and read only the relevant shard:

```bash
funcfinder --dir . --all --json --split
# Read .codemap/manifest.json, then a relevant shard.
funcfinder --dir . --all --json --split --inc
```

Directory JSON has `files[].path` and `files[].functions[]` entries containing `name` and `line`.
Paths are on files, not function objects. Single-file map JSON instead uses names as keys with `start`/`end`.
It is a navigation map, not an editable code object. Same-name entries can collapse in that legacy JSON map;
JSON object reads return explicit ambiguity candidates instead.

```bash
funcfinder --dir . --names
funcfinder --inp file.go --source go --map
funcfinder --inp file.go --source go --doc Name
callgraph --dir . -l go --reverse --func Name
callgraph --inp file.go -l go --func Name --depth 2
```

`--dir` defaults to mapping; ordinary `--inp` mode needs `--source` and an operation.
Language keys: `go`, `c`, `cpp`, `cs`, `java`, `d`, `js`, `ts`, `py`, `rust`, `swift`, `kotlin`, `php`, `ruby`, `scala`, `goose`.
Finding neighbors on one line does not imply exact editing support in that language. `[one-line]` means equal
start/end lines, not necessarily shared code. Text extraction returns covering physical lines and may include neighbors.

## Go/Rust/C#/D/JS/TS/Kotlin/Goose replacement: JSON objects first

```bash
funcfinder --inp file.go --source go --read-object Name
funcfinder --inp file.go --source go --read-object Box --object-kind type
```

The returned `funcfinder.code/v1` object contains exact declaration `source`, opaque `id`, absolute `path`,
`language`, `kind`, `name`, physical `line`/`column`/`end_line`, and whole-file SHA-256 `revision`.
Only change `source`. Do not compute offsets, reconstruct metadata, or maintain a separate `--old` file.
Use a JSON-aware client; never interpolate source into shell command strings.

```bash
funcfinder --edit-object -          # Edited object on stdin; validates without writing.
funcfinder --edit-object - --apply  # Send the SAME request after preview.
```

Go, Rust, C#, D, JavaScript, TypeScript, Kotlin and Goose declarations are supported by this protocol (`--source go|rust|cs|d|js|ts|kotlin|goose`). Read the backend scope/validation limits below. Go aliases, generics and grouped
types work; a group member's `source` includes a synthetic `type` keyword and is converted back automatically.
Leading doc comments remain outside the object. The tool preserves BOM, EOL style and surrounding bytes.
Function names/receiver types and type names must remain unchanged. Syntax/identity checks do not prove that
parameters, fields, callers or behavior preserve the intended contract; run relevant project checks.

Success returns `status`, `changed`, `object`, `validation`. `preview` never writes; its returned object describes
the proposed file, not the current disk revision. Apply the original request. After `applied`, use the returned
`object` for subsequent edits. Saving an unchanged object reports `changed: false`.

Failure returns JSON on stdout with `status: "error"`, `error.code`, `error.message`, and a nonzero exit code:

- `ambiguous`: inspect/select an object from `error.candidates`, then edit that object directly. Same-name methods
  on one line are supported. Alternatively re-read with `--line N --column N` (1-based UTF-8 byte column).
- `stale_revision`: read again and reconcile the change. Never fabricate or replace the revision to force an edit.
  An unrelated edit anywhere in the same file also invalidates the object.
- Invalid identity/JSON/options/source/replacement: fix the request or code and preview again; no write occurred.

Object modes reject incompatible legacy flags. `--json` is optional; output is already JSON. Input can also be a
JSON filename. For the complete schema, in-memory automation example and error codes, read
[references/edit.md](references/edit.md) when implementing a client or diagnosing a refusal.

## Go/Rust/C#/D/JS/TS/Kotlin/Goose insertion, removal and transactions

Check `--edit-object` help for single-file batch support before using these newer envelopes. Do not change read
object metadata. For insertion/removal, keep the anchor object unchanged:

```python
insert = {"operation": "insert_after", "object": anchor, "source": "func Helper() {}"}
remove = {"operation": "remove", "object": anchor}
replace = {"operation": "replace", "object": edited_object}
batch = {"protocol": "funcfinder.edits/v1", "edits": [replace, insert]}
```

Send an operation or batch to the same `--edit-object -` command; preview, then send the original request with
`--apply`. Read all batch anchors from one file snapshot. Do not apply each edit sequentially or update revisions
by hand: the tool validates all edits together and writes once. Cross-file batches and overlapping/conflicting
edits are refused. A replaced anchor can receive an insertion in the same batch. Combine multiple insertions
after one anchor into one source. Groups accept inserted types only; top-level insertion accepts functions/types.
Check reverse callers before removal; documentation comments outside the AST span are retained.

Results are in request order: each has `operation`, original anchor `id`, final file `revision` and `objects`.
Replacement returns one updated object, insertion returns the new objects, removal returns an empty list.
All objects use the final revision; previously read unrelated objects are stale and must be re-read. Direct
unwrapped replacement keeps its single `object` response. A failed batch writes nothing. Run project tests after
successful application; this transaction checks syntax/declarations, not call-site or type compatibility.
Use explicit UTF-8 for subprocess JSON input/output on Windows. For complete examples/error codes read
[references/edit.md](references/edit.md).

## Rust backend limits

Use the same objects and edit/batch envelopes with `--source rust`. Functions/methods in module, impl, trait and
extern scopes, including bodyless trait/extern declarations, and struct/enum/union/trait/type-alias objects are
recognized by byte tokens. Outer attributes are part of source and must be preserved; leading docs stay outside.
Raw identifiers keep their `r#` spelling. Macro token trees and function bodies are opaque for item discovery;
macro-generated declarations and local functions are not selectable. Read external module files separately.

Rust reports `rust_tokens_and_declaration_identity`, not compiler validation. It checks token balance, recognized
item boundaries and lexical scopes; it does not fully parse Rust expressions/grammar, resolve types/receivers,
evaluate cfg, or expand macros. Run cargo/project checks. Unsupported shapes are refused; do not bypass the
revision/identity guards or interpret a successful preview as a compilation guarantee. Insertion must respect its
impl/trait/extern context. Parent type/trait and child-method edits overlap and cannot share a batch.

## C# backend limits

Use `--source cs` and the same JSON objects/operations/batches for methods, constructors, destructors, operators,
conversions and bodyless declarations in named types, plus class/struct/interface/record/enum type objects.
Namespaces, nested types, overload ambiguity, attributes and same-line neighbors retain byte spans. Select overloads
by returned candidate or line/byte column; preserve attributes, explicit interface qualification and constructor category.
Properties/fields/accessors/events/delegates, local functions and top-level statements are not objects. Conditional
directives and assembly/module attributes refuse the file; nonconditional nullable/pragma/region/line directives are
retained outside editable objects. No identifier-escape or semantic overload/type resolution is performed.
`csharp_tokens_and_declaration_identity` means structural checks, not complete C# grammar or Roslyn validation.
Run `dotnet build` and project tests. See [references/edit.md](references/edit.md) for limits and measured examples.

## D backend limits

Use `--source d` and the same objects/operations/batches for module functions, aggregate methods, constructors,
destructors, property/template functions and named aggregate/enum/modern alias declarations. Prefixes/UDAs and
post-parameter attributes must remain unchanged. Contracts and literal/comment forms retain exact byte spans;
overloads return candidates. Attribute/access scopes and explicit templates are noneditable containers. Function
and unittest bodies are opaque; their local declarations are not targets. Variables, legacy aliases, manifest
constants, invariants and unittest/mixin containers are not objects. Conditional/mixin declaration scopes and
line directives refuse the file. No template expansion, semantic overload/type resolution or full D grammar check
is performed. `d_tokens_and_declaration_identity` is structural validation: compile with LDC/DMD and run project
tests. See [references/edit.md](references/edit.md) for the supported profile and independent measurements.

## JS/TS backend limits

Use `--source js|ts` for named functions, direct variable arrows/function expressions, named methods/accessors,
constructors and direct function-valued fields/object properties, plus class/interface/enum/type-alias objects.
Preserve decorators/prefixes. Variable objects expose synthetic complete declarations: for `const a=()=>1,b=()=>2;`,
`b` has source `const b=()=>2;`, but coordinates point at the original binding. Keep the original const/let/var;
the backend handles list commas and sole-binding removal. Do not derive physical offsets from this synthetic source.
Function bodies/call arguments are opaque; local functions/callbacks, computed names, anonymous declarations and
wrapped function values are outside the selectable profile. JSX/templates/regex are lexical islands. Mixed EOL
refuses writes. `javascript_typescript_tokens_and_declaration_identity` is structural, not a complete parser or
type checker: run project compilation/tests. Read [references/edit.md](references/edit.md) for exact limits/evidence.

## Kotlin backend limits

Use `--source kotlin` for named top-level/member/extension `fun` declarations and named classes/interfaces/objects/
companions/typealiases. Block/expression bodies, generic/suspend/operator functions and bodyless signatures use
exact code-token spans. Preserve annotations, modifiers and context/extension/generic receiver prefixes. Select
overloads by candidate or physical line/byte column. Unnamed companions are scopes with selectable named members.
Kotlin also accepts `--object-kind property` for named val/var (including delegates) and `--object-kind accessor`
for explicit get/set, including a written visibility-only setter. Properties own their accessors: parent/child edits
overlap. Replace a property to add its first accessor. Accessor insertion accepts only get/set in that property;
val refuses setters, duplicates are rejected, and repeated get/set names require candidate/coordinate selection.
Preserve val/var, receiver prefixes, annotations and accessor visibility. Primary-constructor property parameters,
destructuring, secondary constructors, initializers and enum entries are not separate objects. Initializer/function/
accessor and enum-entry bodies are opaque; local declarations and anonymous-object members there are outside this profile. Leading KDoc/comments stay outside source. Mixed EOL refuses edits; literal/scope nesting is
bounded. `kotlin_tokens_and_declaration_identity` is structural, not complete grammar/type/compiler validation.
Run kotlinc/Gradle and project tests. See [references/edit.md](references/edit.md) for exact scope and PSI evidence.

## Goose backend limits

Use `--source goose` for top-level fn/thread_fn, standalone extern signatures and struct/enum/type aliases.
Keep the written name/qualification, declaration category, function modifiers and extern/export symbols.
Nested functions, fields/globals and extern-block members are not individual objects; edit the owning function/type.
Navigation has a broader scope. Namespace/import/global/extern-block entries protect neighbors in transactions.
The usual JSON read → change only source → preview → apply workflow works without new flags or span preparation.
`goose_tokens_and_declaration_identity` is structural: run `goose check FILE` and relevant tests after applying.
See [references/edit.md](references/edit.md#goose-json-objects) for scope, exact-byte evidence and native checks.

## Legacy editing and other languages

If object mode is unavailable, disclose that capability limit and use guarded text editing where supported:

```bash
funcfinder --inp file.py --source py --func Name --extract
funcfinder --inp file.py --source py --replace-func Name --old old.txt --with new.txt
funcfinder --inp file.py --source py --replace-func Name --old old.txt --with new.txt --apply --show
```

Keep the exact current source as `--old`; a stale refusal means re-read and reconsider, never omit the guard.
The extraction header is accepted, but inspect EOF padding if the expected text fails to match.
`--with -` reads replacement text from stdin. `--line N` disambiguates names where possible.
Use `--add-after NAME` / `--remove-func NAME` for function insertion/removal. Go types have corresponding
`--replace-type`, `--add-after-type`, `--remove-type`; `--old` applies to replacement only.
Check reverse callers before removal. Other-language spans and safety checks vary; keep explicit refusals and
consult [references/edit.md](references/edit.md), then run the language/project tests.

## Other toolkit operations

```bash
funcstats file.go -l go -j
complexity --inp file.go -l go
deps . -l go --shards --no-gitignore --update-manifest .codemap/manifest.json
```

When `deps`/`callgraph` miss gitignored source directories such as `cmd/`, use `--no-gitignore` if those files are
within the investigation scope. Shard dependency resolution needs the application root/module configuration.
Do not clone, publish or delete repositories merely to follow this skill; use the user's authorized scope.
