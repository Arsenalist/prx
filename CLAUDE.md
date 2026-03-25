# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Test Commands

```bash
go build -o prx .                  # Build binary
go test ./...                      # Run all tests
go test ./internal/metrics/...     # Run tests for a specific package
go test -run TestCalculate ./internal/metrics/...  # Run a single test
go test -v ./...                   # Verbose test output
```

No linter is configured. No Makefile or task runner — use `go` commands directly.

## Architecture

**prx** is a Go CLI (Cobra) that fetches GitHub PR data, stores it in SQLite, and computes developer productivity metrics.

### Key layers

- `main.go` → `cmd/` — CLI commands via Cobra. `root.go` handles DB/config resolution. Each command file (`fetch.go`, `analyze.go`, `report.go`, etc.) is self-contained.
- `internal/provider/` — VCS provider abstraction. `interface.go` defines `VCSProvider`. Only GitHub is implemented (`github/client.go`).
- `internal/store/` — Storage abstraction. `interface.go` defines the `Store` interface with all DB operations. Only SQLite is implemented (`sqlite/`).
- `internal/sync/` — Smart fetch engine. Handles incremental sync (skips closed/merged PRs already in DB), test file classification via regex patterns, and branch comparison for timing data.
- `internal/metrics/` — Analytics calculator. `Calculate()` takes PRs + DB reader, produces `AnalysisResult` with summary, per-developer stats, and slowest PRs. Uses `RawQuery` for file-level LOC breakdown.
- `internal/report/` — Output formatters: table, JSON, markdown.
- `internal/config/` — YAML config loading (legacy `prx import` path) and DB-based settings (`LoadSettings`).
- `internal/hooks/` — Post-analyze hook execution.

### Data flow

`fetch` → `sync.Engine` calls `VCSProvider` → stores in `Store` (PRs, file changes, branch info, timeline events)
`analyze` → reads PRs from `Store` → `metrics.Calculate()` → `report` formatters → stdout/file

### DB resolution order

1. `--db` flag → 2. `$PRX_DB` env → 3. `~/.config/prx/prx.db`

### Configuration

All config lives in the SQLite DB as key-value settings (no YAML). Managed via `prx config set/get/list/reset`. The `prx import` command migrates legacy YAML configs.

### Dependencies

Go 1.26+, Cobra (CLI), modernc.org/sqlite (pure-Go SQLite, no CGO), testify (tests), gopkg.in/yaml.v3 (legacy import only).
