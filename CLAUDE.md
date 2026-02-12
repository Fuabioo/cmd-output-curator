# coc — CMD Output Curator

## Testing

**UNBREAKABLE RULE**: All tests MUST run inside an isolated Docker container.
NEVER run `go test` directly on the host. Always use `just test` or the
Dockerfile.test container. This prevents test side effects (temp files,
signal handling, process spawning) from affecting the host system.

## Build

```bash
just build          # Build the binary
just test           # Run tests in Docker
just test-verbose   # Run tests with verbose output in Docker
just lint           # Run golangci-lint
just install        # Install to GOPATH/bin
```

## Module

- Module path: `github.com/Fuabioo/coc`
- Entry point: `main.go`
- CLI framework: Cobra

## Architecture

See `docs/ARCHITECTURE.md` for the full package dependency graph.

Key packages:
- `internal/cli` — Cobra commands, flag parsing
- `internal/executor` — MultiWriter tee engine, signal forwarding
- `internal/filter` — Strategy interface, registry, all filters
- `internal/logpath` — Log path resolution, slug/session ID generation

## Conventions

1. Go-like error handling: never ignore errors
2. Use `crypto/rand` for random values, not `math/rand`
3. Version is injected via ldflags (`-X internal/cli.Version`, `-X internal/cli.Commit`)
4. Only external dependency: `github.com/spf13/cobra`
5. Filters implement `filter.Strategy` interface and register in the registry
6. Log files go to `$TMPDIR/coc/<slug>/<session-id>.log`
7. Stderr is always passthrough (never filtered)
8. Exit code always preserved from child process
