---
name: funcfinder
description: Map repositories, locate and extract functions/types, trace calls and dependencies, and edit Go declarations through versioned JSON code objects. Use for code navigation, impact analysis and function/type replacement with the funcfinder toolkit.
---

# funcfinder

Use the toolkit to locate declarations before reading source. Treat a function or type as the editing unit;
reason about its contract and replacement code, leaving byte selection and stale-file checks to the tool.
Skill use does not authorize changing unrelated repositories, publishing, or messaging others.

## Locate the current binaries

The five tools are `funcfinder`, `funcstats`, `deps`, `callgraph`, `complexity` (add `.exe` on Windows).
Check `funcfinder --help` for `--read-object` and `--edit-object` before choosing the JSON workflow.
Do not infer capability from the version number alone or use an older learning snapshot.

On this user's Windows machine, the maintained checkout is `D:\Dev\Go\funcfinder` and the binary is
`D:\Dev\Go\funcfinder\funcfinder.exe`. Use this absolute binary while working in another repository.
If rebuilding is needed, build the active funcfinder checkout with `build.ps1` on Windows or `build.sh` on Unix.
Do not run a funcfinder build inside the target application's repository. JSON editing may not yet exist in
the remote default branch; check actual help rather than silently substituting an older installed binary.

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
Language keys: `go`, `c`, `cpp`, `cs`, `java`, `d`, `js`, `ts`, `py`, `rust`, `swift`, `kotlin`, `php`, `ruby`, `scala`.
Finding neighbors on one line does not imply exact editing support in that language. `[one-line]` means equal
start/end lines, not necessarily shared code. Text extraction returns covering physical lines and may include neighbors.

## Go replacement: JSON objects first

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

Only Go functions/methods and top-level types are supported by this protocol. Aliases, generics and grouped
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
