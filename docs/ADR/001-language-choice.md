# ADR-001: Language Choice — Go

## Status
Accepted

## Context
coc needs to proxy CLI commands, tee output to log files, and filter stdout. We evaluated Go and Rust.

## Decision
**Go** over Rust.

## Rationale

| Factor | Go | Rust |
|--------|-----|------|
| MultiWriter pattern | `io.MultiWriter` + `io.TeeReader` in stdlib | Requires manual `impl Write` or crate |
| Streaming I/O | Goroutines + io.Pipe | Tokio adds complexity |
| Build speed | 1-3s incremental | 20-60s incremental |
| GoReleaser | Native, `CGO_ENABLED=0` | Requires cargo-zigbuild shim |
| Binary size | ~8MB | ~4MB |
| Command overhead | 8-20ms | 5-15ms |

**Decision driver**: The core value of coc is the MultiWriter tee — `io.TeeReader` + `io.MultiWriter` make this ~5 lines in Go. The performance delta is negligible since the bottleneck is the underlying command, not the proxy.

## Consequences
- Module: `github.com/Fuabioo/coc`
- Entry: `cmd/coc/main.go`
- CLI: Cobra
- Release: GoReleaser v2 + Homebrew tap
