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
Go 1.15.9** (`funcfinder-windows-amd64.exe` of the GitHub release; `ffd_compare.py` prints and records the reference's
version). Go defaults to all 32 threads, `ffd` to 8 (at most 16). Milliseconds; `@1` and `@8` give both tools the same
number of threads:

| Tree | Go @1 | ffd @1 | Go @8 | ffd @8 | Go default | ffd default |
|---|---:|---:|---:|---:|---:|---:|
| gson (Java) | 99 | 51 | 53 | 39 | 46 | 39 |
| fmt (C++) | 224 | 65 | 72 | 59 | 75 | 58 |
| polly (C#) | 205 | 107 | 95 | 72 | 91 | 73 |
| zod (TS) | 226 | 149 | 65 | 49 | 54 | 49 |
| Seal (Kotlin) | 108 | 37 | 38 | 28 | 31 | 29 |
| arsd (D) | 562 | 230 | 112 | 99 | 128 | 101 |
| flask (Python) | 44 | 25 | 21 | 23 | 21 | 23 |
| huginn (Ruby) | 140 | 93 | 45 | 56 | 53 | 57 |
| maybe (Ruby) | 164 | 92 | 68 | 70 | 57 | 66 |
| serde (Rust) | 62 | 40 | 26 | 32 | 25 | 31 |
| Alamofire (Swift) | 209 | 55 | 55 | 47 | 45 | 48 |
| Kingfisher (Swift) | 190 | 40 | 55 | 31 | 39 | 31 |
| OmniRoute (TS/JS) | 2917 | 2061 | 655 | 452 | 483 | 447 |
| tdtp-framework (Go) | 241 | 134 | 72 | 88 | 59 | 88 |

**Per thread `ffd` does much less work** (serially 29% to 79% less time, 45-79% on most trees) and **scales worse than Go**:
Go goes about 6 times faster from 1 to 32 threads, `ffd` about 4.5 times from 1 to 8 and not at all beyond (OmniRoute, before
the scanners below: 559 ms at 8 threads, 870 at 16, 1463 at 32). With 8 threads each `ffd` is ahead on 9 of 14 trees, level on
1, behind on 4; against Go's default of 32 threads it is ahead on 8 and behind on 6. The limit is the shared runtime
allocator and the finders that still allocate; the next steps are in `TODO.md`.

The TypeScript `declaration_pattern` and `declarator_pattern` are read by scanners (as `func_pattern` is): OmniRoute serial
2643 → 2061 ms, with 8 threads 611 → 452 ms; zod serial 206 → 149 ms. The Go tool has the same scanners in its own PR.

The tokenizers and cleaners of every finder keep their token, pair, stack and text blocks from file to file on each thread
(`reuse.d`), and the D finder keeps the text of its cleaned lines in one block. Parse time at 1 → 8 workers (ms, the same
outputs byte for byte on 30 trees × 4 modes): arsd (D) 213 → 189 serially and 82 → 39 with 8 threads (2.6× → 4.9×), Seal (Kotlin)
23 → 10 and 13 → 3, serde (Rust) 28 → 12 and 18 → 4, Alamofire (Swift) 35 → 21 and 28 → 10, huginn (Ruby) 74 → 51 and 33 → 14,
gson (Java) 36 → 23 and 24 → 7, OmniRoute 1906 → 1834 and 290 → 274 (6.6×). Allocation fell from 8-20 to 1-8 bytes per source
byte in most finders; what is left is the JS finder's phases on small files, C++ (35 on tdtp) and Ruby (7.7).
