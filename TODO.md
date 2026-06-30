# TODO

Known issues and follow-up work, tracked here until they become GitHub issues.

---

## Bugs

### HybridStructFinder: brace-less `type_alias` swallows following types (TS/JS)

**File:** [internal/struct_finder_factory.go](internal/struct_finder_factory.go) — `findAllTypes` (around line 159)

**Found while:** adding test coverage for `struct_finder_factory.go`
(`internal/struct_finder_factory_test.go`).

**Symptom:** A single-line TypeScript `type` alias with no braces, e.g.:

```ts
type Handler = (req: Request) => void;

class Server {
    start() {}
}
```

causes `Server` to never be reported as its own type. `Handler` ends up
"open" indefinitely and absorbs everything after it (including `Server`)
until some later brace pair drives the running `depth` counter from `>0`
back to `0` — at which point *that* event closes the alias, not the class
that triggered it.

**Root cause:** in `findAllTypes`, when a type is opened with `braceCount ==
0` (no `{` on the same line) and the language is not `IndentBased`, `depth`
is initialized to `0`. The closing condition is `depth == 0 && prevDepth >
0` — but since `depth` starts at `0` and stays there until something opens a
brace, that condition is never satisfied by the alias's own lines. The type
only gets closed by an unrelated later brace-balance event, or — if no such
event ever occurs — by the EOF fallback (`if currentType != nil &&
currentType.End == 0`), which is why a *lone* type alias at end-of-input
works correctly (see `TestHybridStructFinder_TypeAliasWithoutBraces_SingleType`)
but one followed by more code does not (see
`TestHybridStructFinder_TypeAliasWithoutBraces_SwallowsFollowingLines`).

**Suggested fix:** when `braceCount == 0` and the language is not
`IndentBased`, the type is by construction single-line — close it
immediately the same way the `braceCount > 0 && depth == 0` branch already
does, instead of leaving it open with `depth = 0`.

**Test coverage:** the buggy behavior is pinned (not silently relied upon)
by `TestHybridStructFinder_TypeAliasWithoutBraces_SwallowsFollowingLines` in
`internal/struct_finder_factory_test.go`. When this is fixed, that test's
expectation should be updated to assert both `Handler` and `Server` are
found as separate types.

**Impact:** Affects `funcfinder --struct` / `--struct --extract` output
quality on TypeScript/JavaScript codebases that mix brace-less type aliases
with classes/interfaces/enums — type aliases are common in real TS code, so
this can silently drop types from `--struct` output on affected files.
