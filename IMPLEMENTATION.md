# prx — Implementation Plan

> Testable milestones for building prx from SPEC.md.
> Each milestone has a **done-when** criteria — a concrete command you can run to verify it works.

---

## Milestone 0: Project Scaffold

**Goal:** Empty project compiles, runs, prints help.

**Tasks:**
- [x] `go mod init`
- [ ] Install dependencies: `cobra`, `yaml.v3`, `modernc.org/sqlite`
- [ ] Create directory structure:
  ```
  prx/
  ├── cmd/                    # Cobra command definitions
  │   ├── root.go             # Root command, global flags
  │   ├── init.go
  │   ├── fetch.go
  │   ├── analyze.go
  │   ├── summarize.go
  │   ├── report.go
  │   ├── status.go
  │   ├── teams.go
  │   ├── export.go
  │   └── db.go
  ├── internal/
  │   ├── config/             # YAML config loading + validation
  │   │   ├── config.go       # Types + loader
  │   │   ├── defaults.go     # Default values
  │   │   └── validate.go     # Validation rules
  │   ├── provider/           # VCS provider abstraction
  │   │   ├── interface.go    # Provider interface
  │   │   └── github/         # GitHub implementation
  │   │       ├── client.go
  │   │       ├── pulls.go
  │   │       ├── branches.go
  │   │       └── timeline.go
  │   ├── store/              # Storage abstraction
  │   │   ├── interface.go    # Store interface
  │   │   └── sqlite/         # SQLite implementation
  │   │       ├── sqlite.go   # Open/Close/Migrate
  │   │       ├── schema.go   # SQL migrations
  │   │       ├── pulls.go    # PR CRUD
  │   │       ├── repos.go    # Repo/instance CRUD
  │   │       └── metadata.go # Fetch metadata
  │   ├── metrics/            # Metrics calculation
  │   │   ├── calculator.go
  │   │   ├── timing.go
  │   │   ├── developer.go
  │   │   └── classifier.go   # Test file classification
  │   ├── report/             # Output formatting
  │   │   ├── table.go
  │   │   ├── json.go
  │   │   └── markdown.go
  │   ├── hooks/              # Hook execution
  │   │   └── hooks.go
  │   └── sync/               # Smart fetch/sync engine
  │       └── sync.go
  ├── main.go                 # Entry point
  ├── go.mod
  ├── go.sum
  ├── SPEC.md
  ├── IMPLEMENTATION.md
  └── prx.example.yaml
  ```
- [ ] Stub all commands (just print "not implemented yet")
- [ ] `main.go` wires up cobra root command

**Done when:**
```bash
go build -o prx . && ./prx --help
# Shows: prx - PR analytics for engineering teams
# Lists all subcommands: init, fetch, analyze, summarize, report, status, teams, export, db

./prx fetch --help
# Shows fetch flags: --team, --repo, --instance, --full, --dry-run, etc.
```

---

## Milestone 1: Config System

**Goal:** Load, validate, and display a YAML config file.

**Tasks:**
- [ ] Define config structs (instances, teams, repos, fetch, output, storage, hooks, test_patterns)
- [ ] YAML loader with config file resolution order (--config, ./prx.yaml, $PRX_CONFIG, ~/.config/prx/config.yaml)
- [ ] Environment variable substitution (`${VAR_NAME}`)
- [ ] Validation: repo format, required fields, instance references, regex compilation
- [ ] `prx init` generates a starter prx.yaml (interactive in TTY, template in non-TTY)
- [ ] Example config file: `prx.example.yaml`

**Done when:**
```bash
./prx init
# Creates prx.yaml in current directory

cat prx.yaml
# Valid YAML with all sections commented/populated

./prx init --minimal
# Creates minimal config (just instance + one repo)

# Edit prx.yaml with a real GitHub instance, then:
./prx status
# Prints "Config loaded: 1 instance, 2 repos, 1 team" (or similar)
# Prints "No data fetched yet."

# Test validation:
# Remove token_env from config → clear error message
# Use invalid repo format → clear error message
```

---

## Milestone 2: Storage Layer

**Goal:** SQLite database with full schema, migrations, and CRUD operations.

**Tasks:**
- [ ] Define `store.Store` interface (all methods from SPEC §5.1)
- [ ] SQLite implementation: Open, Close, Migrate
- [ ] Schema v001: all tables from SPEC §5.3 (instances, repos, teams, pull_requests with raw_data, branch_info, file_changes, timeline_events, fetch_metadata, schema_version)
- [ ] Migration runner (auto-run on open, version tracking)
- [ ] CRUD: UpsertInstance, UpsertRepository, UpsertPullRequest, UpsertBranchInfo, UpsertFileChanges
- [ ] Read: GetPRState, ListPullRequests (with filters), GetFetchMetadata, ListRepositories
- [ ] Raw query support (read-only)
- [ ] WAL mode enabled on open
- [ ] Write tests for CRUD operations

**Done when:**
```bash
go test ./internal/store/sqlite/ -v
# All tests pass: insert PR, read back, upsert updates, filters work,
# raw_data JSON preserved, schema migration runs cleanly

./prx db stats
# Prints: "Database: ~/.config/prx/prx.db (empty, 0 PRs)"

./prx db path
# Prints: /Users/zarar/.config/prx/prx.db
```

---

## Milestone 3: GitHub Provider + Fetch

**Goal:** Fetch real PR data from GitHub and store in SQLite.

**Tasks:**
- [ ] Define `provider.VCSProvider` interface (SPEC §6.1)
- [ ] GitHub implementation: auth, base URL, TLS skip
- [ ] ListPullRequests: pagination, state filter, since filter, early stop
- [ ] GetPullRequest: full detail (additions/deletions)
- [ ] GetBranchComparison: using commit SHAs, file changes
- [ ] GetTimelineEvents: extract ready_for_review_at
- [ ] GetRateLimit: remaining check
- [ ] Retry logic: exponential backoff on 5xx/network errors, no retry on 4xx
- [ ] Smart sync engine (SPEC §7.1 decision matrix):
  - Skip closed/merged PRs already in DB
  - Re-fetch open PRs
  - Full fetch for new PRs
- [ ] `prx fetch` command wired up end-to-end
- [ ] Progress display (repo name, PR count, skipped/new/updated)
- [ ] `prx status` shows fetched repos, PR counts, date ranges

**Done when:**
```bash
# Set up config with a real public repo:
export GITHUB_TOKEN=ghp_...

# First fetch:
./prx fetch --repo golang/go
# Output: "Fetched 100 new, 0 updated, 0 skipped PRs for golang/go"

./prx status
# Shows: golang/go — 100 PRs, last fetched just now

# Second fetch (incremental):
./prx fetch --repo golang/go
# Output: "Fetched 0 new, 3 updated, 97 skipped PRs for golang/go"
# (most are closed/merged → skipped)

./prx db stats
# Shows row counts for all tables

# Verify raw_data stored:
./prx db query "SELECT number, json_extract(raw_data, '$.user.login') FROM pull_requests LIMIT 5"
# Returns PR numbers with author logins extracted from JSON blob
```

---

## Milestone 4: Metrics Engine

**Goal:** Calculate all metrics from stored data.

**Tasks:**
- [ ] Timing metrics: time_to_open, draft_time, time_to_merge, total_time
- [ ] Volume metrics: total, merged, closed, open, unique authors
- [ ] Size metrics: total/avg/median LOC, additions, deletions
- [ ] File classifier: test vs production LOC using configurable regex patterns
- [ ] Developer metrics: per-author breakdown (PRs/week, avg LOC, avg timings)
- [ ] Multi-repo grouping
- [ ] Date range filtering (applied at query level)
- [ ] Write tests with fixture data

**Done when:**
```bash
go test ./internal/metrics/ -v
# All tests pass with known fixture data producing expected metric values

# Using data fetched in Milestone 3:
./prx analyze --repo golang/go --preset last-30d --format json | jq '.summary'
# Returns JSON with: total_prs, merged_prs, unique_authors, loc, timing

./prx analyze --repo golang/go --preset last-30d --format json | jq '.developers[:3]'
# Returns top 3 developers with prs_per_week, avg LOC, timing metrics
```

---

## Milestone 5: Output Formatters

**Goal:** Table, JSON, and markdown output fully working.

**Tasks:**
- [ ] JSON formatter (canonical schema from SPEC §9.3) — stdout
- [ ] Table formatter (summary, developer, slowest PRs tables) — stderr when piped, stdout otherwise
- [ ] Markdown formatter (file output with naming convention)
- [ ] `--format all` mode (table + JSON + markdown simultaneously)
- [ ] `prx analyze` fully wired
- [ ] `prx summarize` command (business-friendly JSON output)
- [ ] `prx report` command (fetch + analyze combined)

**Done when:**
```bash
# Table output (human-friendly):
./prx analyze --repo golang/go --preset last-30d --format table
# Prints formatted summary, developer, and slowest PRs tables

# JSON output (agent-friendly, pipeable):
./prx analyze --repo golang/go --preset last-30d --format json | jq .
# Valid JSON with meta, summary, developers, slowest_prs, repo_breakdown

# Markdown output (file):
./prx analyze --repo golang/go --preset last-30d --format markdown
# Creates reports/golang-go-2026-03-25.md

# Combined fetch + analyze:
./prx report --repo golang/go --preset last-week
# Fetches (incremental), then prints table analysis

# Pipe to another tool:
./prx report --repo golang/go --preset last-week --format json | wc -l
# Produces valid JSON on stdout (table goes to stderr)
```

---

## Milestone 6: Teams + Multi-instance

**Goal:** Team-based workflows across multiple GitHub instances.

**Tasks:**
- [ ] `prx teams` — list teams from config
- [ ] `prx teams show <name>` — show team repos
- [ ] `--team` flag works on fetch, analyze, summarize, report
- [ ] Multi-instance: fetch from different GitHub instances in one run
- [ ] Team-level aggregation in metrics
- [ ] Repo breakdown table in output

**Done when:**
```bash
# Config with 2 instances and a team defined:
./prx teams
# Lists: "platform (3 repos across 2 instances)"

./prx report --team platform --preset last-month
# Fetches from both instances, analyzes together, shows repo breakdown

./prx analyze --team platform --group-by repo --format table
# Shows per-repo metrics within the team
```

---

## Milestone 7: Hooks + Export + Polish

**Goal:** Extensibility, data portability, production readiness.

**Tasks:**
- [ ] Hooks engine: discover from config, pipe JSON to stdin, timeout, error handling
- [ ] post-fetch and post-analyze hook events
- [ ] `prx export` command (JSON, CSV)
- [ ] `prx db raw <repo> <pr#>` — dump raw JSON blob
- [ ] Date presets: all named presets + rolling windows (SPEC §11)
- [ ] TTY detection: no color, no spinners, no prompts in non-TTY
- [ ] `--verbose` and `--quiet` modes
- [ ] `--dry-run` for fetch
- [ ] Progress bars for fetch

**Done when:**
```bash
# Hook test:
# Add to prx.yaml:
#   hooks:
#     post-analyze:
#       - name: "word-count"
#         command: "wc -c"
./prx analyze --repo golang/go --preset last-30d
# Prints "[hook:word-count] 4523" (or similar byte count)

# Export:
./prx export --repo golang/go --format csv --output prs.csv
cat prs.csv | head -3
# CSV with headers and PR data

# Date presets:
./prx analyze --repo golang/go --preset this-quarter --format json | jq '.meta.date_range'
# Shows correct quarter boundaries

# Non-TTY:
./prx report --repo golang/go --preset last-week --format json 2>/dev/null | jq '.summary.total_prs'
# Clean JSON, no progress bars or color codes
```

---

## Milestone 8: Release + Distribution

**Goal:** Cross-platform binaries, documentation, ready for open source.

**Tasks:**
- [ ] GoReleaser config (.goreleaser.yaml)
- [ ] Build targets: linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64
- [ ] Version injection at build time (`prx --version`)
- [ ] Homebrew formula
- [ ] README.md with quick start (3 steps)
- [ ] prx.example.yaml with full comments
- [ ] LICENSE (MIT or Apache 2.0)
- [ ] CONTRIBUTING.md
- [ ] GitHub Actions CI: test, lint, build on every PR

**Done when:**
```bash
goreleaser release --snapshot --clean
# Produces binaries in dist/ for all platforms

./dist/prx_darwin_arm64/prx --version
# Prints: prx v0.1.0 (commit abc1234, built 2026-03-25)

# Fresh machine test (no config):
./prx init --minimal
# Edit config with a token + repo
./prx report --preset last-week
# Works end-to-end on first try
```

---

## Dependency Choices

| Need | Package | Why |
|------|---------|-----|
| CLI framework | `github.com/spf13/cobra` | Industry standard, subcommands, flag parsing, help generation |
| YAML config | `gopkg.in/yaml.v3` | Stdlib-quality, no unnecessary abstractions |
| SQLite | `modernc.org/sqlite` | Pure Go, no CGO = easy cross-compilation, static binaries |
| HTTP | `net/http` (stdlib) | GitHub REST API is simple; no need for a client library |
| Tables | `github.com/olekukonez/tablewriter` | Simple, works well for CLI output |
| Color | `github.com/fatih/color` | Auto-detects TTY, respects NO_COLOR |
| Testing | `github.com/stretchr/testify` | Assertions + test suites |
| JSON | `encoding/json` (stdlib) | Standard, no extras needed |
| Time | `time` (stdlib) | Go's time package handles everything we need |

---

## Implementation Order Rationale

```
M0 (scaffold) → M1 (config) → M2 (storage) → M3 (fetch) → M4 (metrics) → M5 (output) → M6 (teams) → M7 (polish) → M8 (release)
```

Each milestone builds on the previous and is **independently testable**. You can ship a useful tool after M5 (single-instance, single-team usage). M6-M8 are enterprise/polish features.

Estimated effort per milestone (for an agent like Claude Code or Codex):
- M0: ~30 min (scaffold + stubs)
- M1: ~1 hour (config is well-specified)
- M2: ~2 hours (schema + CRUD + tests)
- M3: ~3 hours (API integration, smart sync — most complex milestone)
- M4: ~2 hours (calculations + tests)
- M5: ~2 hours (formatters + wiring)
- M6: ~1 hour (mostly wiring existing pieces)
- M7: ~2 hours (hooks, export, polish)
- M8: ~1 hour (goreleaser, docs)
