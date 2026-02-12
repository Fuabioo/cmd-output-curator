# CLI Specification

## Usage

```
coc [flags] <command> [args...]
```

## Global Flags

| Flag | Description | Default |
|------|-------------|---------|
| `-v, --verbose` | Increase verbosity (stackable: -vv, -vvv) | 0 |
| `--log-dir DIR` | Override log directory | `$TMPDIR/coc` |
| `--no-filter` | Disable filtering, still write log file | false |
| `--no-log` | Disable log file (implies --no-filter) | false |
| `-h, --help` | Show help | — |
| `--version` | Show coc version and commit | — |

## Flag Interaction

`--no-log` implies `--no-filter` because filtered output without a recovery log file means data loss.

## Flag Parsing Boundary

Everything before the first non-flag argument is a coc flag. Everything from the first non-flag argument onward is the proxied command:

```
coc -v --log-dir /tmp git status --short
     ^^^^^^^^^^^^^^^^^^^          ^^^^^^^^
     coc flags                    git flags
                         ^^^ ^^^^^^
                         proxied command
```

## Exit Code

Always matches the child process:
- Command exits 0 → coc exits 0
- Command exits 1 → coc exits 1
- Command killed by signal N → coc exits 128+N
- Command not found → coc exits 127

## Output Behavior

When filtered:
```
$ coc git status
On branch main
M  internal/cli/root.go
2 files modified

Output was reduced, see the full logs at /tmp/coc/git-status/20260212-143022-a1b2.log
```

When passthrough:
```
$ coc echo hello
hello
```

Footer only appears on stderr when `WasReduced == true`.

## Environment Variables

| Variable | Description |
|----------|-------------|
| `COC_LOG_DIR` | Override default log directory |
