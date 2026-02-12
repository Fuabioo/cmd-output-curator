# ADR-004: Log Path Convention

## Status
Accepted

## Context
coc needs a predictable, organized location for log files.

## Decision

### Path Format
```
<base-dir>/<command-slug>/<session-id>.log
```

### Base Directory Priority
1. `--log-dir` CLI flag
2. `COC_LOG_DIR` environment variable
3. Default: `os.TempDir() + "/coc"`

### Slug Generation
Command name + first non-flag subcommand, lowercased, hyphen-joined:
- `git status` → `git-status`
- `cargo build` → `cargo-build`
- `go test ./...` → `go-test`
- `ls -la` → `ls`

### Session ID
`YYYYMMDD-HHMMSS-XXXX` where XXXX is 4 random hex chars via `crypto/rand`.

Example: `20260212-143022-a1b2.log`

## Consequences
- No config file in Phase 1 (deferred to Phase 4)
- No auto-cleanup in Phase 1 (deferred to Phase 4: `coc clean`)
- 4-hex collision space (65536/sec) is sufficient when scoped by slug directory
