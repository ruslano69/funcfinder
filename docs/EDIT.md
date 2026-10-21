# Editing one function: `--replace-func`, `--add-after`, `--remove-func`

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
- `--old` must match the current text, ignoring line endings and one trailing newline. A mismatch means the file
  changed since it was read.

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
