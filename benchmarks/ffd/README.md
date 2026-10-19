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

Median of 5 to 7 interleaved runs of `--dir --all --json` on a 16-core/32-thread Windows machine, against the **released
Go 1.15.9** (`funcfinder-windows-amd64.exe` of the GitHub release). Go defaults to all 32 threads, `ffd` to 8 (at most 16).
Milliseconds; `@1` and `@8` give both tools the same number of threads:

| Tree | Go @1 | ffd @1 | Go @8 | ffd @8 | Go default | ffd default |
|---|---:|---:|---:|---:|---:|---:|
| gson (Java) | 101 | 53 | 56 | 41 | 47 | 40 |
| fmt (C++) | 224 | 66 | 70 | 59 | 74 | 57 |
| polly (C#) | 206 | 113 | 100 | 74 | 93 | 71 |
| zod (TS) | 228 | 206 | 67 | 73 | 54 | 70 |
| Seal (Kotlin) | 110 | 39 | 39 | 28 | 31 | 28 |
| arsd (D) | 559 | 232 | 114 | 101 | 126 | 99 |
| flask (Python) | 43 | 25 | 21 | 23 | 21 | 24 |
| huginn (Ruby) | 142 | 93 | 46 | 61 | 53 | 56 |
| maybe (Ruby) | 165 | 94 | 69 | 70 | 57 | 68 |
| serde (Rust) | 62 | 40 | 27 | 32 | 24 | 31 |
| Alamofire (Swift) | 208 | 56 | 56 | 51 | 43 | 47 |
| Kingfisher (Swift) | 195 | 39 | 55 | 32 | 40 | 31 |
| OmniRoute (TS/JS, 1.6M lines) | 2871 | 2643 | 642 | 611 | 488 | 610 |
| tdtp-framework (Go, many small files) | 239 | 136 | 71 | 95 | 60 | 92 |

What the table says: **per thread `ffd` does much less work** (serial 8% to 80% less time, 45-80% on most trees), but it
**scales worse than Go**: Go's run goes about 6 times faster from 1 to 32 threads, `ffd`'s about 4 times from 1 to 8 and
not at all beyond (OmniRoute: 559 ms at 8, 870 at 16, 1463 at 32). With 8 threads each `ffd` is ahead on 8 of 14 trees,
level on one, behind on five; against Go's default of 32 threads it is ahead on 6 and behind on 8. The limit is the shared
runtime allocator and the finders that still allocate, and in TypeScript regexes (about 40% of its serial time): the next
steps are in `TODO.md`.

> Correction: the first version of this table, and the "faster than Go's 32 threads on most trees" in the commit message
> of this branch, were measured against a stale local `funcfinder.exe` (version 1.15.7 plus the accuracy fixes, without the
> 1.15.8/1.15.9 speed-ups), not the release. The keys compared are unchanged (same functions and types), the speed claims
> were wrong.
