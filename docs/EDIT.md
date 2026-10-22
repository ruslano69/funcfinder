# Editing code objects and declarations

## Agent JSON protocol (Go, Rust, C#, D, JavaScript and TypeScript functions/types)

Read a declaration as a complete editable object, change only `source`, then send that object back:

```bash
funcfinder --inp utils.go --source go --read-object joinPaths > object.json
# Edit only object.json's source string using a JSON-aware client.
funcfinder --edit-object object.json
funcfinder --edit-object object.json --apply
# A client can stream the object directly instead:
# funcfinder --edit-object - --apply
```

`--read-object NAME` returns the object directly, without a wrapper or extraction header. `--object-kind type`
selects a named type (Go includes top-level aliases, generics and grouped members); the default is `function`.
`--line N` and `--column N` select a declaration by physical start line and 1-based UTF-8 byte column.
Go, Rust, C# (`--source cs`), D (`--source d`), JavaScript (`--source js`) and TypeScript (`--source ts`) are supported. Rust/C#/D/JS/TS use token/scoped-item validation rather than Go AST validation; see their backend sections below. Legacy map/JSON formats remain unchanged.

The v1 object contains `protocol: "funcfinder.code/v1"`, opaque `id`, absolute `path`, `language`, `kind`, `name`,
`line`, `column`, `end_line`, whole-file SHA-256 `revision`, and editable `source`. Source uses LF and contains
exactly the declaration, excluding preceding doc comments and neighboring code. A grouped type is returned as
a complete `type Name ...` declaration; replacement converts it back into its original group. JS/TS variable-bound function objects similarly expose a complete synthetic variable declaration; see below. Keep all metadata
unchanged. No separate `--old`, range manipulation or temporary source files are needed.

A client can perform the entire read/preview/apply cycle in memory. This example assumes `file.go` contains
`func Count() int { return 0 }` and intentionally replaces its implementation:

```python
import json
import subprocess

exe = "./funcfinder"  # Use "./funcfinder.exe" on Windows.

def run(args, obj=None):
    result = subprocess.run(
        [exe, *args],
        input=None if obj is None else json.dumps(obj),
        capture_output=True, text=True, encoding="utf-8",
    )
    response = json.loads(result.stdout)
    if result.returncode:
        raise RuntimeError(response["error"])
    return response

obj = run(["--inp", "file.go", "--source", "go", "--read-object", "Count"])
obj["source"] = "func Count() int { return 42 }"
preview = run(["--edit-object", "-"], obj)
# Inspect preview["object"]["source"]. Apply the SAME request after review.
applied = run(["--edit-object", "-", "--apply"], obj)
obj = applied["object"]  # Current identity/revision for the next edit.
```

Replacement reads a JSON file, or stdin for `--edit-object -`. It rejects unknown JSON fields, extra documents,
unsupported protocols, altered identities, stale revisions, mixed endings and structurally invalid replacements. Full grammar validation depends on the backend.
No other editing/output modes can be combined with object mode; `--json` is optional because output is always JSON.
Go functions retain their name and receiver type; types retain their name. Rust retains item name, kind and outer attributes. These checks do not enforce parameter,
field, return-type or behavioral compatibility. Run project compilation/tests for that contract.

Success returns `status: "preview"|"applied"`, `changed`, the resulting `object`, and
`validation: "go_syntax_and_declaration_identity"`. Preview never writes. Its returned object describes the
**proposed** file: apply the original edited request, not the preview response. After application, use the returned
`object` as the starting point for the next edit. Saving an unchanged object returns `changed: false`.

Errors return `status: "error"` and `error: {code, message}` with a nonzero exit status. `ambiguous` also returns
`error.candidates`: complete objects that can be edited directly, including same-name methods on one line.
`stale_revision` requires reading again and reconsidering the change; do not replace the revision to bypass it.
The whole-file revision deliberately invalidates the object even after an unrelated edit elsewhere in the file.

Client decisions should use `error.code`, not parse `message`:

| Code | Client action |
|---|---|
| `ambiguous` | Select an object from `error.candidates`; inspect its source before changing it. |
| `stale_revision` | Read again and reconcile with the new file; do not retry with a fabricated revision. |
| `not_found`, `invalid_selector` | Correct the name, kind or physical coordinates. |
| `invalid_identity` | Read a new object instead of reconstructing metadata. |
| `invalid_json`, `invalid_options` | Fix the JSON envelope or incompatible flags. |
| `invalid_source`, `invalid_replacement`, `invalid_result`, `contract_changed` | Inspect the code/diagnostic; nothing was written. |
| `unsupported_language`, `unsupported_protocol`, `invalid_kind` | Use supported Go/Rust/C#/D/JS/TS v1 function/type objects. |
| `mixed_endings`, `invalid_path`, `io_error` | Correct the file/environment before retrying. |

Application stages and syncs a temporary file in the source directory, rechecks the original bytes, then renames
it over the source. Regular files only; symlink targets are refused. This protects against partial writes but is
not a cross-process lock: coordinate simultaneous writers. Original BOM, EOL style and bytes outside the selected
span are preserved. Doc comments outside the span remain untouched; grouped conversion refuses declaration
comments outside the replacement spec rather than silently losing them.

## Insert, remove and batch edits (Go, Rust, C#, D, JavaScript and TypeScript)

The same `--edit-object FILE|- [--apply]` command also accepts an operation wrapper:

```python
insert = {"operation": "insert_after", "object": anchor,
          "source": "func Helper() int { return 42 }"}
remove = {"operation": "remove", "object": old_object}
replace = {"operation": "replace", "object": edited_object}
```

`insert_after` and `remove` take an **unchanged** object read from disk. Insertion source belongs on the wrapper;
replacement source belongs in `object.source`. Removal takes no wrapper source. An insertion may contain multiple
functions/types after a top-level declaration; inside a type group it may contain only types. Original surrounding
bytes and group wrappers remain intact. Ordinary declarations get blank-line separation; shared lines use explicit
separators. Removal consumes the declaration and its following explicit semicolon, keeping surrounding comments
and whitespace. Leading doc comments remain outside the object. Check callers before deleting declarations.

For a refactoring within one file, read all anchors from the same file revision, construct a batch and submit it once:

```python
original = run(["--inp", "file.go", "--source", "go", "--read-object", "Count"])
edited = dict(original)
edited["source"] = "func Count() int { return Helper() }"
batch = {
    "protocol": "funcfinder.edits/v1",
    "edits": [
        {"operation": "replace", "object": edited},
        {"operation": "insert_after", "object": original,
         "source": "func Helper() int { return 42 }"},
    ],
}
preview = run(["--edit-object", "-"], batch)
applied = run(["--edit-object", "-", "--apply"], batch)
```

All objects must name the same absolute file and current whole-file revision. Edits refer to the original snapshot,
not intermediate results. Request order does not determine splice order: the tool orders original byte spans itself.
One anchor can be replaced and receive an insertion in the same transaction. Duplicate replacements, conflicting
replace/remove operations, overlapping spans, or multiple separate insertions at one anchor are refused; combine
insertion declarations into one source. Inserted top-level names must not introduce duplicates. These are syntax/name
checks, not full compiler resolution of receivers, aliases, imports or references.

The complete resulting file is checked with its language backend and its declaration order verified before any write. A bad operation anywhere
rejects the **whole** batch; apply writes once through the staged-file mechanism above. This guarantee is for one file
only: batches spanning multiple files are refused. No automatic call-site updates or project tests are performed.

Wrapped and batch responses return `status`, `changed`, `validation`, and `results` in **request order**. Each result
contains its `operation`, original anchor `id`, final file `revision`, and `objects`: one refreshed object for replacement,
all newly inserted objects for insertion, or an empty array for removal. All returned objects share the final revision.
Other objects read before the batch are now stale; read those again. Direct unwrapped replacement retains its existing
single `object` response for compatibility. Preview results describe proposed state, so still apply the original request.

Additional error codes: `invalid_operation` for malformed wrappers; `empty_batch`; `multiple_files`;
`overlapping_edits`; `duplicate_declaration`. All use the same JSON error envelope and nonzero exit code.

## Rust JSON objects

```bash
funcfinder --inp src/lib.rs --source rust --read-object format
funcfinder --inp src/lib.rs --source rust --read-object Data --object-kind type
# Change source, then preview/apply the same object, operation or batch:
funcfinder --edit-object request.json
funcfinder --edit-object request.json --apply
```

Rust uses the same `funcfinder.code/v1` objects, revisions, ambiguity candidates and single-file
`funcfinder.edits/v1` transaction envelopes. Coordinates are original UTF-8 byte positions; an item with outer attributes starts at its first attribute. Named items in
modules, inherent/trait `impl` blocks, traits and `extern` blocks are supported. Function declarations ending
in `;` are accepted in trait/extern context; methods with bodies and ordinary functions are supported.
Type objects cover structs (named/tuple/unit), enums, unions, traits and type aliases, including associated aliases.
Raw identifiers retain their `r#` spelling. Generics/lifetimes and same-line neighbors retain original spans.

Outer `#[...]` attributes belong to the object; replacement must preserve their token content. Preceding doc
comments remain outside the object. Literals (including raw/byte/C strings and chars) and nested block comments
are lexed before delimiter pairing. Macro token trees are opaque: definitions generated by macros are not
objects. Function bodies are opaque for item discovery, so local items inside a function are not selectable.
External `mod name;` source files must be read separately. Module/impl containers themselves are not editable
objects. A trait object owns its nested declarations; editing it and a nested method in one batch is an overlap.
Insertion into impl/trait/extern contexts must respect their item kinds; type definitions such as structs cannot
be inserted into associated-item lists. Macro expansion, cfg evaluation and receiver/type resolution are not performed.

Rust success reports `validation: "rust_tokens_and_declaration_identity"`. This backend verifies lexical delimiter
balance, recognized item boundaries, scope and the final item catalogue. **It is not a complete Rust grammar or
compiler check**, and unsupported shapes can be refused as `invalid_source`/`invalid_replacement`. Balanced but
invalid body expressions, type errors, visibility/edition constraints and macro semantics can still pass these
structural checks. Run `cargo check`/project tests, or `rustc --edition=2024` on a standalone example afterward.
Go responses retain `validation: "go_syntax_and_declaration_identity"`.

Verification includes token/scope regressions (`go test ./internal -run TestRustCodeObject`), a five-operation
Rust 2024 transaction compiled before/after with runtime output 20 -> 22, and a real itoa `Buffer::format`
local-variable rename compiled as a library before/after with six formatting cases (including u128/i128 bounds).
These examples establish their tested behavior, not full Rust-language coverage. The legacy text-edit commands
and navigation parser keep their existing limits; use Rust JSON mode for exact structural edits.

Grammar/token reference: [Rust functions](https://doc.rust-lang.org/reference/items/functions.html) and
[Rust tokens](https://doc.rust-lang.org/reference/tokens.html).

## C# JSON objects

```bash
funcfinder --inp Counter.cs --source cs --read-object Count
funcfinder --inp Counter.cs --source cs --read-object Counter --object-kind type
funcfinder --edit-object request.json
funcfinder --edit-object request.json --apply
```

C# uses the same objects, revision checks, operation wrappers and single-file batches. Methods, constructors,
destructors, operators, conversion operators and bodyless interface/abstract declarations are selectable inside
named types. Named types cover classes, structs, interfaces, records (including bodyless records) and enums,
including nested types. Block/file-scoped namespaces retain lexical ownership. Expression bodies include their
terminating semicolon. Exact UTF-8 byte spans preserve neighbors on the same line, BOM and consistent EOL style.

Attributes belong to the object and must retain their token content on replacement. Explicit interface qualification
and static-constructor category are preserved. Leading doc comments remain outside. Overloads return ambiguity
candidates; choose a returned object or its physical line/byte column. Duplicate checks distinguish syntactic
parameter types and generic arity, ignoring parameter names/default values; they do not resolve aliases, equivalent
CLR signatures or generic type-parameter renamings. Semantic overload validity remains a compiler responsibility.

Member fields, properties, indexers, events, delegates, accessors, local functions and top-level statements are not
editable objects in this backend. Method bodies/property bodies are opaque for item discovery. Conditional and
unsupported directives (including `#if`/`#define`) and assembly/module attributes refuse the file. Nonconditional
`#nullable`, `#pragma`, `#region`, `#endregion` and `#line` directives are retained as noneditable catalogue entries;
an insertion cannot introduce a directive outside its new declaration. Unicode identifier escapes are not resolved.
Normal/verbatim/raw/interpolated strings, chars and comments are lexed before pairing structural delimiters.

Success reports `validation: "csharp_tokens_and_declaration_identity"`. This is structural validation, **not a full
C# grammar, Roslyn compilation or type check**. Balanced invalid expressions may pass. Run `dotnet build` and project
tests after editing. Scope nesting over 128 and literal nesting over 64 are refused. No reference updates or multi-file
transactions are performed. Go/Rust response contracts and legacy C# text commands retain their existing behavior.

Verification: `go test ./internal -run TestCSharpCodeObjects` covers source spans, attributes, overload selection,
transactions, stale/forged identities, overlap, directives, literals and EOL preservation. Independent .NET SDK 9.0.308 /
Roslyn 4.14 checks matched all 21 objects in a standalone program before/after a three-operation transaction; compilation
and execution changed output from 2 to 20. A source-order sample of ten Dapper files (42 objects) and ten Polly files
(47 objects) matched Roslyn source/coordinates and unchanged replacement previews. This is a bounded sample, not
whole-project or full-language editing coverage. Local logs are under ignored `build/_csharp-objects-20261022/`.

## D JSON objects

```bash
funcfinder --inp source.d --source d --read-object count
funcfinder --inp source.d --source d --read-object Buffer --object-kind type
funcfinder --edit-object request.json
funcfinder --edit-object request.json --apply
```

D uses the same versioned objects, operation wrappers and one-file transactions. Module functions, aggregate methods,
constructors/destructors (`this`/`~this`), property functions, templated functions and bodyless declarations are supported.
Type objects cover named structs, classes, interfaces, unions, enums and `alias Name = ...` declarations. Manifest
constants (`enum value = ...`) are not types. Explicit templates and attribute/access blocks or sections retain lexical
ownership as noneditable catalogue entries; their supported nested declarations remain selectable.

Exact byte spans retain neighbors on one line, outer declaration prefixes/UDAs, contracts (`in`/`out`, `body`/`do`),
template constraints, expression bodies, BOM and consistent EOL style. Prefixes and post-parameter attributes must be
preserved on replacement. Doc comments remain outside. Overloads return candidates selected by physical line/byte
column or returned identity. Duplicate checks use syntactic parameter/template arity and qualifier keys, not resolved
aliases, equivalent types, template specialization or semantic constraint evaluation.

The lexer handles nested `/+ +/` comments, ordinary comments, escaped/character/raw/backtick/hex strings, token strings
`q{...}` and paired/identifier/delimited `q"..."` strings before structural delimiter pairing. Function and `unittest`
bodies are opaque for object discovery: their local functions, local types and declarations generated there are not
selectable. Variables, explicit template containers, mixin-generated declarations, invariants and unit tests are not
editable objects. Legacy alias forms are not type objects. Conditional/mixin declaration scopes (`version`, `debug`,
`static if`, `mixin`) and line directives refuse the file; this backend does not expand or select their branches.

Success reports `validation: "d_tokens_and_declaration_identity"`: lexical/scope/catalogue checks, **not full D grammar,
compiler semantics or template instantiation**. Balanced invalid expressions can pass. Compile with LDC/DMD and run
project tests after applying edits. Scope nesting over 128 and literal nesting over 64 are refused. The common
transaction planner and Go/Rust/C# response contracts remain unchanged.

Verification: `go test ./internal -run TestDCodeObjects` covers spans, constructors, overloads, contracts/templates,
literal forms, type operations, opaque scopes, stale/forged identities, no-mutation failures and EOL preservation.
LDC 1.43.0 (DMD frontend 2.113.0), `-mcpu=x86-64` and the initialized VS2022 x64 environment compiled a standalone
program before/after a three-operation transaction; output changed from 2 to 20. Compiler JSON verified 20 function
end coordinates and the presence of seven named types/aliases in each version; this does not establish independent
type-end accuracy. A bounded tree-sitter D sample checked 35 arsd functions and 17 D-YAML functions across eight
accepted files per project, with unchanged replacement previews and no span mismatches. Nine arsd files and one
D-YAML file were excluded for grammar errors, unsupported scopes or the 200 KiB sample limit; local functions in
opaque bodies were explicitly excluded. These samples are not full-project/full-language coverage. Corpus pins,
grammar/binary hashes, exclusions and local logs are in ignored `build/_d-objects-20261022/`.

## JavaScript and TypeScript JSON objects

Use `--source js` or `--source ts` with the same read/edit objects and single-file transactions. Supported objects
include named function declarations, direct variable-bound arrows/function expressions, named class/interface
methods and signatures, constructors, getters/setters, private/string-named methods and direct function-valued
fields or object properties. Classes, interfaces, enums and type aliases are type objects. Namespaces, ambient
global augmentations, top-level control blocks and directly assigned object literals retain lexical ownership as
noneditable containers. Decorators and declaration prefixes belong to the selected span and must be retained.

Variable-bound objects use the binding name, including a named function-expression initializer. In
`const a=()=>1, b=()=>2, c=()=>3;`, reading `b` returns `source: "const b=()=>2;"` with physical coordinates at `b`.
This complete declaration is synthetic: replacement strips the keyword/terminal semicolon and splices only the
selected binding. Keep its original `const`/`let`/`var`. Insertion accepts compatible same-keyword bindings and
adds commas; removal preserves the remaining list and removes its outer declaration only for a sole binding.
Object methods/properties similarly preserve comma separators. Never calculate splice offsets from the length
of synthetic `source`. Adjacent removals whose separator spans overlap are refused as one batch.

Byte tokens handle comments, quoted/template/regexp literals and nested JSX before pairing structural delimiters.
UTF-8 columns, shared-line neighbors, BOM and consistent EOL style are preserved. Function bodies and call/new
arguments are opaque for object discovery: local declarations and callbacks are outside this profile. Anonymous
declarations, computed member names, class expressions, destructured bindings and arrows wrapped in assertions or
other expressions are not selectable objects; unsupported enclosing shapes may refuse a file. This backend does
not resolve imports/types, evaluate decorators or macros, or validate semantics. TypeScript overload candidates
use syntactic signatures; duplicate checks are not compiler overload resolution.

Success reports `javascript_typescript_tokens_and_declaration_identity`: structural lexical/scope/catalogue
checks, **not complete JavaScript/TypeScript grammar or type checking**. Balanced invalid expressions can pass.
Run the target project's compiler, lint and tests after edits. Scope nesting over 128 and JSX nesting over 64 are
refused. Navigation finders retain their existing behavior; these checks apply to JSON editing.

Verification: `go test ./internal -run TestScriptCodeObjects` covers spans, shared-line binding/member CRUD,
TSX, type syntax, ambient scopes/imports/re-exports, stale/forged metadata, no-mutation refusals and EOL/BOM.
An independent TypeScript 5.8.3 AST comparison on hr_gemini revision
`8c6f613c04b2502fc2672f2494d5dc8a49b06907` matched all 3698 objects across 465 tracked JS/TS/JSX/TSX files
in the stated profile, with no missed/extra/span differences or excluded files. All-object identity replacements
produced 3674 unchanged previews; 24 were explicitly refused for mixed EOL in three files. Separate CLI transactions
for standalone JS and TS changed runtime output from 2 to 20, preserved a third same-line arrow and refused stale
reapplication. Actual comment edits on disposable copies of `scripts/audit-code.ts` and `src/App.tsx` passed
TypeScript parsing/transpilation; the application repository and its database were not modified or executed.
These measurements describe the unreleased working-tree implementation and this corpus/profile, not universal
language coverage, full type checking or application test success. The reproducible oracle is
`benchmarks/script_objects_oracle.cjs`; hashes, reports and CLI logs are in ignored `build/_js-objects-20261022/`.

## Legacy text commands

These commands change one function in a file. The span comes from the same finder as `--extract`, so an edit does
not touch the code around the function. Every command previews by default and writes only with `--apply`.

```bash
# Replace (the --old file must hold the function's current text; see below)
funcfinder --inp file.py --source py --replace-func add --with new.txt --old old.txt --apply
# Insert new functions directly after one
funcfinder --inp file.go --source go --add-after Add --with new.txt --apply
# Delete one function with its decorators and nested functions
funcfinder --inp file.go --source go --remove-func Add --apply
```

`--with -` reads the new text from standard input, so no shell quoting is involved. `--line N` picks one function
when several share the name; `N` is the start line that `--map` reports.

## Guards against common slips

- A first line that `--extract` prints (`// name: start-end` or `# name: start-end`) is removed from `--with` and
  from `--old`. A markdown fence (```` ``` ````) around the code is removed; a fence line inside the code is refused.
- `--source` must match the file's extension. A `.py` file with `--source go` is refused before anything runs.
- A UTF-8 byte order mark at the start of the file is kept, including when line 1 is the one replaced.
- Without `--apply` the preview prints the removed lines with `-` and the added lines with `+`, so the change can be
  read before it is written.

## Showing the result

`--show` prints the lines the edit put in place, with their numbers. After `--apply` they are read back from the file,
so the output is what is on disk; in a preview they come from the new text. A removal says so.

```bash
funcfinder --inp file.go --source go --replace-func Add --with new.txt --apply --show
```

## What is checked

- The finder must find the target exactly once.
- The finder must report the same functions and types, in the same order, in the new file, apart from the change.
  A rename, a duplicate or a missing function is refused.
- Brace languages must balance their brackets in the new text. Strings and comments are removed first.
- Decorators and annotations above a function (`@x`, `#[x]`) belong to it. A replacement without them keeps the old
  ones. A different set is refused.
- Python: the new text must start at the function's indentation, its `def` must have a deeper body, no line may
  leave the block, only declarations and comments may sit at the function's level, and tabs and spaces may not be
  mixed. Lines inside triple-quoted strings are exempt.
- Go: the span must agree with `go/ast`, and the new file must parse with the same top-level declarations.
- Line endings are kept. A file that mixes `\r\n` and `\n` is refused.
- Go functions sharing a physical line use exact AST byte spans. Replace, insert and remove preserve neighboring
  bytes, including comments; removal consumes only the following explicit separator semicolon, if present.
  The lexer skips literals and comments. The resulting file must parse and retain declaration order and method
  receivers. `--old` accepts either the exact function fragment or its complete covering physical lines.
  Duplicate names on the same line remain ambiguous and are refused.
- Other languages still require complete lines without neighboring code.
- The input is read before the file, so the file is as it is when the edit is prepared. Before `--apply` the file
  is read again, and the command writes only if it is unchanged.
- `--old` must match the current text, ignoring line endings and one trailing newline. A mismatch means the file
  changed since it was read.

## Legacy type commands (Go only)

`--replace-type NAME --with FILE|-`, `--add-after-type NAME --with FILE|-` and `--remove-type NAME`
edit a top-level Go type selected by AST, including structs, interfaces, named types, aliases and generics.
`--line N` selects the declaration start; `--old FILE` guards replacement exactly as for functions.
Other languages are refused. Insertion accepts only types and rejects duplicate top-level names.

Types sharing a line and members of `type ( ... )` groups use exact byte spans. A grouped edit preserves the
surrounding `type` keyword, parentheses and neighboring specs. Supply replacement/insertion text as complete
`type Name ...` declarations; the editor converts them to specs inside a group. For grouped replacement,
`--old` accepts the original spec without `type`, or the complete covering physical lines. Comments outside
replacement specs are refused when converting into a group, to avoid silently relocating or discarding them.
Existing comments outside the selected span remain unchanged.

The edited file must parse and preserve neighboring declarations. This does not check compatibility of fields,
methods, type parameters, aliases or callers: compile the project and run its behavioral tests afterward.

## Limitations

These limits describe legacy text commands; the JSON backend sections above define their separate contracts.

- **Only the finder's view is validated, except for Go.** For languages other than Go the tool trusts the finder's
  spans and its list of functions and types. A wrong span from the finder carries over into the edit. The known
  finder gaps are listed in `TODO.md`, for example C++ conversion operators and macro-generated declarations.
- **Python has no parser.** Indentation is checked by rule, not by compiling. A `return` placed at class level, a
  name used before it is defined, or a call with wrong arguments is not caught. Run the project's own tests or
  `python -m py_compile` after the edit.
- **No reference check.** Removing a function does not look for its callers. Run
  `callgraph --dir . -l <lang> --reverse --func <Name>` first.
- **Comments above the `func` line are not part of the span.** A removed function leaves its doc comment behind;
  a replacement keeps it. Doc comments are not edited by these commands.
- **One target per legacy command.** Changing two targets needs two commands. Fields and fragments inside a
  function are not targets. Go/Rust/C#/D/JS/TS JSON batches can edit multiple supported declarations in one file.
- **Nested functions go with their parent on removal.** Replacing a parent does not keep its nested functions
  unless the new text contains them, so the finder check refuses that case.
- **Decorators cannot be removed by a replacement.** A replacement can only restate the old set or keep it. Drop
  decorators with a separate edit once that is supported.
- **Trailing blank lines are not part of a span.** Removal also takes the blank lines after the function. Some
  finders report a function's end past the end of the file; the tool clamps it.
- **`--extract` prints a header line.** Its first line is `// name: start-end`. Strip it with `tail -n +2` before
  using the output as `--old` or as the new text.
- **Other attribute syntaxes are not recognised.** C# `[Attr]` and TypeScript decorators without a leading `@` are
  not kept when a replacement omits them.
- **Not atomic across files.** Each command edits one file.
- **A window remains before the write.** The file is re-read just before writing, but another process can still
  change it between that read and the write. There is no file lock.
- **One-line labels.** Single-file function maps print `[one-line]` when start and end are equal; their JSON
  includes `single_line: true`. The label describes the span, not whether neighboring code shares the line.
  Extraction headers retain their original format for `--old`.
- **Discovery and editing have different limits.** Mapping can report multiple functions on one line in other
  languages too. Extraction still returns their covering physical lines, which can include neighbors.
- **Shared-line JSON editing supports Go, Rust, C#, D, JavaScript and TypeScript.** Legacy text commands retain their language-specific restrictions.
  Go validation checks syntax and declaration identity, not type correctness or runtime behavior; run project tests.
