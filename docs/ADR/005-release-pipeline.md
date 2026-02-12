# ADR-005: Release Pipeline

## Status
Accepted

## Context
coc needs automated CI and release workflows.

## Decision

### CI (on push to main + PRs)
1. Build: `go build ./...`
2. Test: Docker-isolated via `Dockerfile.test`
3. Lint: `golangci-lint`
4. Security: `govulncheck ./...`

### Release (on `v*` tag push)
1. Checkout with `fetch-depth: 0`
2. Run tests
3. GoReleaser `release --clean`

### GoReleaser
- Targets: `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`
- `CGO_ENABLED=0` for static binaries
- ldflags inject Version and Commit
- Homebrew tap: `Fuabioo/homebrew-tap`

### Version Injection
```go
var Version = "dev"
var Commit  = "unknown"
```
Set via ldflags: `-X github.com/Fuabioo/coc/internal/cli.Version={{.Version}}`

## Consequences
- Tests always run in Docker (never on host)
- Version bumps via git tags
- Homebrew formula auto-updated by GoReleaser
