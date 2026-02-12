# Architecture

## Package Dependency Graph

```
main.go
    └── internal/cli
            └── internal/executor
                    ├── internal/filter
                    └── internal/logpath
```

No circular dependencies. Each package has a single responsibility.

## Package Responsibilities

| Package | Responsibility | Key Types |
|---------|---------------|-----------|
| `main.go` | Entry point | — |
| `internal/cli` | Cobra commands, global flags, version | `rootCmd`, `Version`, `Commit` |
| `internal/executor` | MultiWriter tee, command execution, signal forwarding | `Config`, `Result`, `Run()` |
| `internal/filter` | Strategy interface, registry, all filters, ANSI stripping | `Strategy`, `Registry`, `Result` |
| `internal/logpath` | Log path resolution, slug, session ID | `Resolve()`, `CreateLogFile()` |

## Data Flow

```
coc <command> [args...]
        │
        ▼
   CLI (parse flags, extract proxied command)
        │
        ▼
   Executor (start child, set up tee, forward signals)
        │
        ├── stdout → TeeReader → log file (raw, real-time)
        │                      → buffer → filter → os.Stdout
        │
        ├── stderr → MultiWriter → log file + os.Stderr
        │
        └── wait → exit code → footer (if reduced) → os.Exit
```

## Dependencies

```
github.com/spf13/cobra   # CLI framework
```

No other external dependencies.
