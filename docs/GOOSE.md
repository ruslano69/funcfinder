# Goose

Available since **funcfinder 1.16.1**. The Go toolkit recognizes `.goose` files with language key `goose`.
Build the current checkout with `build.ps1` or `build.sh` before use.

```bash
funcfinder --inp main.goose --source goose --map
funcfinder --inp main.goose --source goose --func main --extract
funcfinder --inp main.goose --source goose --struct --map
funcfinder --dir my-project --all --json --split
funcstats main.goose -l goose --json
callgraph --dir my-project -l goose --reverse --func process
deps my-project -l goose -j
complexity main.goose -l goose -j
```

## Discovery scope

- Named `fn` and `thread_fn` declarations, including nested functions, generic headers, modifiers, multiple declarations on one line and external declarations ending with `;`.
- Named `struct`, `enum` and `type` aliases. Struct field names are listed; enum variants and semantic field types are not analyzed.
- Explicit qualified names are reported as written (`ns::name`, `::name`). A file's `namespace` does not implicitly rewrite reported names.
- Line extraction preserves original source and covers complete physical lines. Shared-line extraction can include neighbors; it is not an exact editable byte object.
- Nested `/* */` comments, line comments, escaped strings/chars and `"""` strings are masked before tokens are recognized. Literal payloads never introduce declarations, even in a function finder's raw mode.

The scanner follows Goose `src/lexer.h` (`Next`, `LexRawString`, `LexEscapedChar`) and `src/parser.h` (`ParseTopDeclInner`, `ParseFnDecl`, `ParseStructDecl`, `ParseEnumDecl`) at upstream revision `94cc64d`. It is declaration navigation, not a full compiler: it does not enforce type/lifetime rules, validate raw-string indentation, expand compile-time constructs, specialize functions or fetch packages. Anonymous function values are not named functions. Invalid/incomplete code has no full-parser correctness guarantee.

Callgraph and funcstats use the toolkit's lexical call model, not semantic overload/generic resolution. `deps` lists written import module paths, including relative and sourced imports; it does not resolve Goose packages or reproduce Goose's module search path. Goose-specific shard dependency resolution is not implemented. JSON editing of top-level functions/types uses the existing code-object protocol; see [Goose JSON objects](EDIT.md#goose-json-objects) for scope and validation limits.

The experimental D tools do not yet implement Goose. These results concern the shipped Go tools only.

## JSON editing

```bash
funcfinder --inp main.goose --source goose --read-object main
funcfinder --inp codec.goose --source goose --read-object Range --object-kind type
funcfinder --edit-object request.json
funcfinder --edit-object request.json --apply
```

Change only `source`; preview and apply the same request. Replacement, insertion, removal and single-file
batches are supported. Top-level functions/extern signatures and struct/enum/alias types are editable.
Nested functions and extern-block members remain navigation targets only. Function modifiers and binding
symbols are guarded; globals/imports/namespace/extern blocks protect neighboring scopes. Validation is
structural (`goose_tokens_and_declaration_identity`); run Goose compiler and project checks.

An independent exact-byte reference matched 7,478 objects and unchanged previews across the same 855
accepted files below. Four grammar-error exclusions remain explicit. These are top-level JSON objects, a
narrower scope than navigation. Reproduction and native editing experiments: [editing guide](EDIT.md#goose-json-objects).

## Independent verification

Goose callgraph assigns calls by token body ranges, so shared-line functions and nested function bodies keep distinct callers. This is covered by explicit expected-edge regressions, independently of the declaration corpus measurement below.

`test_examples/test_example.goose` passes the compiled Goose compiler's `check` command. Go regressions cover nested comments/literals, modifiers, generic/default headers, extern declarations, qualified names, nested/shared-line functions, type aliases, extraction offsets and call-count signature exclusion.

`benchmarks/goose_tree_eval.py` scores named functions and types against Goose's own independently generated Tree-sitter grammar. It requires a supplied runtime and grammar shared library; it installs nothing and executes no application code.

On Goose `94cc64d`, tracked `.goose` files under `stdlib/`, `samples/`, `bench/` and `test/` (excluding `test/errors/` and `test/errors_tc/`) give 859 input files. Four grammar-error files are explicit exclusions. Across the 855 accepted files, all 6,384 functions and 1,498 types match names, header start ranges and end lines, with zero missing/extra entries or wrong ends. This does not measure exact byte spans, field accuracy, package resolution or semantic correctness.

Reference runtime: tree-sitter 0.25.2; upstream grammar generated with tree-sitter-cli 0.25.8. Example invocation from the repository root:

```bash
python benchmarks/goose_tree_eval.py --root build/research/goose --binary ./funcfinder.exe --runtime build/accuracy-runtime --grammar-library build/research/goose-grammar.dll --output build/research/goose-language-eval.json
```

The output retains source hashes, grammar/runtime information, corpus revision, binary hash, per-file truth, exclusions and differences. Generated evidence belongs under ignored `build/`.

## Project boundary

This repository owns Goose parsing, toolkit integration, editor guards, fixtures and
independent accuracy evaluators. Standalone compression libraries and applications
are maintained in [lz78-goose](https://github.com/ruslano69/lz78-goose), including
its [archiver subtree](https://github.com/ruslano69/lz78-goose/tree/main/archiver).
They are external editing examples, not runtime or test dependencies of funcfinder.
