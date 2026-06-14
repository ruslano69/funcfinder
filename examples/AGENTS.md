# examples/

## Purpose

Shell script usage examples and swe-agent integration workflows showing end-to-end funcfinder usage patterns.

## Ownership

- `analyze.sh` — general-purpose analysis example.
- `swe-agent/` — SWE-agent and mini-SWE-agent config and workflow scripts.

## Local Contracts

- Scripts must reference binaries by their short names (`./funcfinder`, `./stat`, etc.) and assume `build.sh` has been run.
- `swe-agent/miniswe-config.yaml` configures the mini-SWE-agent integration; update it when the funcfinder CLI interface changes.

## Work Guidance

- When adding an example, prefer showing a complete workflow (orient → find → extract → analyze) rather than isolated commands.
- Keep scripts executable (`chmod +x`).

## Verification

```bash
./build.sh && bash examples/analyze.sh
```

## Child DOX Index

- `examples/swe-agent/` — SWE-agent config and bug-fix / refactor workflow scripts
