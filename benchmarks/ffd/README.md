# ffd: funcfinder's directory run in D

`ffd` reads a source tree with the token finders of all 15 languages that live next to it (`../*_port.d`), in parallel,
and prints what `funcfinder --dir` prints. It is the first executable of the D port that is more than a probe; the
contract and the rules for changing it are in [AGENTS.md](AGENTS.md).

```bash
python benchmarks/ffd/build.py                       # build/ffd/ffd[.exe]; needs LDC (F:\D\ldc-1.43.0 or $LDC2)
build/ffd/ffd --dir . --all --json                   # functions, types and Go aliases, as JSON
build/ffd/ffd --dir src --struct                     # types only, path:line: name
build/ffd/ffd --dir . --all --json --workers 4 --timing
```

Implemented: `--dir`, `--all`, `--struct`, `--json`, `--workers` (default 8, at most 16), `--recursive=false`,
`--no-gitignore`, `--timing`, `--version`. The root `.gitignore`, hidden names and the extension table follow the Go
tool exactly (the same files, the same "skipped N files" notice on 31 trees). Not yet: `--inp`, `--tree`, `--names`,
`--doc`, `--split`, `--extract`, `funcstats`, `callgraph`, `complexity`, `deps`.

## How close to Go

Measured by key (`ffd_compare.py`), Go 1.15.9 against `ffd` on 30 trees:

- **Functions:** all Go functions are found except 185 of about 98,000 (Rust macro bodies, some C++ templates, three in
  Kotlin and Ruby). `ffd` also finds functions Go does not (Python `requests` +29, C headers, Swift, Kotlin).
- **Types:** JavaScript and TypeScript are identical to Go's (`express`, `lodash`, `zod` are byte-identical in `--all`;
  OmniRoute's 6,971 types match). Other languages list their own named types: more than Go for Kotlin, Swift, Scala,
  Rust, Ruby, Python and PHP; fewer for C#, C/C++ and D. Counts per corpus: `ffd_deltas.json`.
- **Bytes:** identical on 6 of 30 trees (Java gson, Express, lodash, zod, D-YAML, Flask); elsewhere the keys match but
  the order of `classes` follows the source, not Go's struct-finder order, or the sets differ as above.

## Speed

Median of 5 interleaved runs, `--dir --all --json`, 16-core/32-thread Windows machine. `ffd` uses 8 threads, Go all 32
(its default):

| Tree | Go | ffd | | Tree | Go | ffd |
|---|---:|---:|---|---|---:|---:|
| OmniRoute (TS/JS, 1.6M lines) | 758 ms | 617 ms | | Kingfisher (Swift) | 54 ms | 33 ms |
| arsd (D, 9.5 MB) | 134 ms | 99 ms | | polly (C#) | 127 ms | 73 ms |
| gson (Java) | 62 ms | 42 ms | | fmt (C++) | 85 ms | 61 ms |
| maybe (Ruby) | 81 ms | 69 ms | | tdtp-framework (Go, small files) | 77 ms | 93 ms |

Run serially (`--workers 1`) `ffd` takes 2.2 s on OmniRoute against Go's 4.1 s, and 236 ms on arsd against 592 ms. The
run stops scaling after about 8 threads (the shared runtime allocator, and regex-based finders that still allocate),
which is why the default is 8 and the cap 16; the next gains are in the finders: TypeScript spends about 40% of its
time in regexes (`declaration_pattern` alone 20%, `declarator_pattern` 7%), the same patterns Go still runs.
