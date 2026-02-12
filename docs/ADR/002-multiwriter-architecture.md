# ADR-002: MultiWriter Architecture

## Status
Accepted

## Context
coc must simultaneously write full output to a log file and deliver filtered output to stdout.

## Decision
Use the TeeReader pattern:

1. **Setup**: Parse args, resolve log path, create log file, look up filter strategy, install signal handler
2. **Execute**: Start child. Stdout → `io.TeeReader(pipe, logFile)` for real-time logging + buffer. Stderr → `io.MultiWriter(logFile, os.Stderr)`.
3. **Wait**: Collect exit code.
4. **Filter**: Apply matched strategy to stdout buffer → `filter.Result{Filtered, WasReduced}`.
5. **Output**: Write filtered stdout to os.Stdout. Raw stderr already delivered.
6. **Footer**: If `WasReduced`, write footer to stderr.
7. **Exit**: `os.Exit(childExitCode)`.

## Signal Handling
Forward SIGINT, SIGTERM, SIGQUIT to child process. If child killed by signal N, exit with 128+N.

## Design Decisions
- Log file gets raw output in real-time (via TeeReader), not post-hoc
- Stderr always passes through unfiltered
- Footer goes to stderr (doesn't pollute pipes)
- Exit code always preserved
- TTY limitation: child sees `isatty(stdout) == false` (acceptable for AI agent use case)

## Failure Modes

| Scenario | Behavior |
|----------|----------|
| Log dir unwritable | Warning on stderr, proceed without log |
| Disk full | Warning on stderr, stdout still delivered |
| SIGPIPE | Standard Go behavior, coc exits |
| SIGKILL | Instant death, log may be incomplete |
| Command not found | Exit 127 |

## Consequences
- Memory buffering: stdout fully buffered before filtering (Phase 1 limitation)
- No streaming filters in Phase 1 (agent sees nothing until command exits)
