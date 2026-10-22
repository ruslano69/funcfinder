# Editing code objects and declarations

## Agent JSON protocol (Go and Rust functions/types)

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
selects a top-level type (including aliases, generics and grouped members); the default is `function`.
`--line N` and `--column N` select a declaration by physical start line and 1-based UTF-8 byte column.
Go and Rust are supported. Rust uses token/scoped-item validation rather than Go AST validation; see the Rust section below. Legacy map/JSON formats remain unchanged.

The v1 object contains `protocol: "funcfinder.code/v1"`, opaque `id`, absolute `path`, `language`, `kind`, `name`,
`line`, `column`, `end_line`, whole-file SHA-256 `revision`, and editable `source`. Source uses LF and contains
exactly the declaration, excluding preceding doc comments and neighboring code. A grouped type is returned as
a complete `type Name ...` declaration; replacement converts it back into its original group. Keep all metadata
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
unsupported protocols, altered identities, stale revisions, mixed endings and invalid replacement syntax.
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
| `unsupported_language`, `unsupported_protocol`, `invalid_kind` | Use supported Go/Rust v1 function/type objects. |
| `mixed_endings`, `invalid_path`, `io_error` | Correct the file/environment before retrying. |

Application stages and syncs a temporary file in the source directory, rechecks the original bytes, then renames
it over the source. Regular files only; symlink targets are refused. This protects against partial writes but is
not a cross-process lock: coordinate simultaneous writers. Original BOM, EOL style and bytes outside the selected
span are preserved. Doc comments outside the span remain untouched; grouped conversion refuses declaration
comments outside the replacement spec rather than silently losing them.

## Insert, remove and batch edits (Go and Rust)

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

These are the limits of the checks. Most of them are not checked at all.

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
- **Only one function per edit.** Changing two functions needs two commands. Members of a class or struct, fields
  and fragments inside a function are not targets yet.
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
- **Shared-line JSON editing supports Go and Rust.** Legacy text commands retain their language-specific restrictions.
  Go validation checks syntax and declaration identity, not type correctness or runtime behavior; run project tests.
