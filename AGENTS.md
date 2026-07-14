# Agents guide

This file gives autonomous agents (AI executors running `/improve` plans)
the information they need to verify their changes.

## Quick verify

```bash
go build ./...        # build everything
go vet ./...          # static analysis
make test             # run tests with race detector
golangci-lint run     # run linter
```

## Language and version

Go 1.26.1 (see `go.mod`). Uses Go modules.

## Package layout

```
minifleet/
├── main.go                    # Entrypoint
├── cmd/                       # Cobra command definitions, flags, filters, fleet resolution
├── internal/
│   ├── provider/              # Provider interface + single GitHub impl
│   │   └── github/
│   ├── git/                   # System git operations (clone, pull, status)
│   ├── fleet/                 # Executor (bounded concurrency) + Scanner
│   ├── manifest/              # YAML manifest load/save/merge (fleet.yml)
│   └── ui/                    # gorich table helpers
```

## Code conventions

- **Error handling**: wrap with `fmt.Errorf("context: %w", err)`. Never bare
  returns. See `cmd/discover.go:52-54` for the pattern.
- **Imports**: stdlib first, blank line, third-party, blank line, internal.
  `gofmt`/`goimports` enforce this (configured in `.golangci.yml`).
- **No unnecessary abstractions**: only `provider.Provider` is an interface;
  everything else is concrete types + function types.
- **Concurrency**: the `fleet.Executor` handles parallelism. Individual
  operations are synchronous closures.
- **Configuration flows**: viper → `cmd.NewConf()` → context → command `RunE`
  via `cmd.confFromCtx()`.
- **Filters**: every repo-operating command shares the `cmd.Filters` struct
  and `cmd.addFilterFlags()`.
- **Manifest-first**: commands use `manifestToTasks` when a manifest exists,
  fall back to API/filesystem scan when it doesn't.

## Tests

- `make test` requires `CGO_ENABLED=1` (it sets it) because `-race` needs cgo;
  running `go test -race ./...` directly with `CGO_ENABLED=0` fails.
- Tests use standard `testing` package + table-driven style.
- Filesystem tests use `t.TempDir()` + `t.Setenv("XDG_CONFIG_HOME", ...)`.
- Git-dependent tests use `t.Skip` when git is absent.
- Never hit the real GitHub API in tests: use
  `internal/provider/testing.FakeProvider` (implements `provider.Provider`).
- Pattern: `internal/fleet/scanner_test.go` (git init in temp dir) and
  `cmd/config_test.go` (temp config with XDG_CONFIG_HOME override).
