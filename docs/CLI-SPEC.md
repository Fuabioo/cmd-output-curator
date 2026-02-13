# CLI Specification

## Usage

```
coc [flags] <command> [args...]
```

## Subcommands

```
  version            Show coc version
  hook               Claude Code PreToolUse hook handler (reads JSON from stdin)
  init               Install coc hook into Claude Code settings
  init --uninstall   Remove coc hook from Claude Code settings
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

## Agent Integration

coc integrates with Claude Code via a PreToolUse hook that transparently wraps supported commands.

### Setup

```bash
coc init              # Install the hook
coc init --uninstall  # Remove the hook
```

### How It Works

1. Claude Code invokes a Bash command (e.g., `git status`)
2. The PreToolUse hook runs `coc hook`, piping the tool input as JSON to stdin
3. `coc hook` checks if the command is supported (git, go, cargo, docker, grep, rg, npm, pip, pip3, yarn)
4. If supported and not a shell pipeline, it returns JSON rewriting the command to `coc git status`
5. Claude Code executes the rewritten command, getting filtered output

### Supported Commands

git, go, cargo, docker, grep, rg, npm, pip, pip3, yarn

Commands with shell operators (|, &&, ||, ;, $(), backticks) are not wrapped.
