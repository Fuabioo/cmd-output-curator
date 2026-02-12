# coc — CMD Output Curator

AI agents consume large amounts of tokens reading CLI output. Most of it is noise — progress bars, ANSI escapes, passing tests, verbose status lines.

`coc` proxies CLI commands, tees their full output to a log file, and delivers curated output to stdout. When output is reduced, a footer tells you where the full logs live. Filters can be aggressive because the full output is always recoverable.

## Install

### From source

```bash
go install github.com/Fuabioo/coc@latest
```

### Homebrew

```bash
brew install Fuabioo/tap/coc
```

### Binary releases

Download from [GitHub Releases](https://github.com/Fuabioo/coc/releases).

## Usage

```bash
coc [flags] <command> [args...]
```

### Examples

```bash
coc git status          # filtered git status
coc go test ./...       # show only failures + summary
coc cargo build         # strip progress noise

coc -v git diff         # verbose mode
coc --no-filter make    # passthrough, still log
coc --no-log make       # pure passthrough
coc --version           # show version
```

### Flags

| Flag | Description |
|------|-------------|
| `-v, --verbose` | Increase verbosity |
| `--log-dir DIR` | Override log directory (default: `$TMPDIR/coc`) |
| `--no-filter` | Disable filtering, still write log file |
| `--no-log` | Disable log file (implies `--no-filter`) |
| `-h, --help` | Show help |

### Exit Code

Always matches the child process exit code. If the child is killed by a signal, exits with 128 + signal number.

### Log Files

Full unfiltered output is written to:

```
$TMPDIR/coc/<command-slug>/<session-id>.log
```

Override with `--log-dir` or `COC_LOG_DIR` env var.

## How It Works

```
child process
    ├── stdout → TeeReader → log file (raw, real-time)
    │                      → buffer → filter pipeline → stdout (curated)
    └── stderr → MultiWriter → log file + stderr (always unfiltered)
```

- **Stdout** is buffered, filtered, then written. The log file gets raw output in real-time via TeeReader.
- **Stderr** always passes through unfiltered. Errors should never be hidden.
- **Footer** appears on stderr only when output was actually reduced.

## Limitations

- **Non-interactive only**: Because coc pipes stdout, `isatty()` returns false for the child. Commands that detect TTY will behave as if piped. This is fine for AI agent usage.
- **Memory buffering**: Stdout is fully buffered before filtering. Commands producing >100MB stdout may cause high memory usage.

## Development

```bash
just build          # build binary
just test           # run tests (Docker-isolated)
just lint           # run linter
just install        # install to $GOPATH/bin
```

## License

MIT
