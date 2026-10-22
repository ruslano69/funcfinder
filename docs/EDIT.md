# Editing code objects and declarations

## Agent JSON protocol (Go functions and types)

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
Go is currently the only supported language. Legacy map/JSON formats remain unchanged.

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
Functions retain their name and receiver type; types retain their name. These checks do not enforce parameter,
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
| `unsupported_language`, `unsupported_protocol`, `invalid_kind` | Use supported Go v1 function/type objects. |
| `mixed_endings`, `invalid_path`, `io_error` | Correct the file/environment before retrying. |

Application stages and syncs a temporary file in the source directory, rechecks the original bytes, then renames
it over the source. Regular files only; symlink targets are refused. This protects against partial writes but is
not a cross-process lock: coordinate simultaneous writers. Original BOM, EOL style and bytes outside the selected
span are preserved. Doc comments outside the span remain untouched; grouped conversion refuses declaration
comments outside the replacement spec rather than silently losing them.

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

## Types (Go only so far)

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
- **Shared-line editing is Go only.** Go functions and top-level types support exact spans; other languages still refuse shared lines.
  Go validation checks syntax and declaration identity, not type correctness or runtime behavior; run project tests.
