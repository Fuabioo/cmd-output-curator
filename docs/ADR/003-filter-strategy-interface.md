# ADR-003: Filter Strategy Interface

## Status
Accepted

## Context
coc needs an extensible filtering system where different commands get different filters.

## Decision

### Interface
```go
type Result struct {
    Filtered   string
    WasReduced bool
}

type Strategy interface {
    Name() string
    CanHandle(command string, args []string) bool
    Filter(raw []byte, command string, args []string, exitCode int) Result
}
```

### Registry
Strategies registered in priority order, first match wins, passthrough fallback.

### Command Derivation
`command = filepath.Base(args[0])`, e.g., `coc git status` → `CanHandle("git", ["status"])`.

### Fail-Safe
Filters run inside `recover()`. Panics fall back to passthrough + log to stderr.

## Built-in Strategies (phased)

| Phase | Strategy | Commands |
|-------|----------|----------|
| 1 | Passthrough | * (fallback) |
| 2 | GitStatus, GitDiff, GitLog | git |
| 2 | GoTest, GoBuild | go |
| 3 | CargoTest, CargoBuild | cargo |
| 3 | DockerPs | docker |
| 3 | ProgressStrip | npm, docker pull |

## Consequences
- All packages are `internal/`, so interface can evolve freely
- Phase 1 only ships passthrough
