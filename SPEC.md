# prx — Specification Document

> A CLI tool for analyzing pull request data across repositories, teams, and GitHub instances. Designed for enterprise-wide engineering metrics, status reports, and agent-consumable structured output.

**Version:** 1.0 (Draft)
**Status:** Pre-implementation

---

## Table of Contents

1. [Overview](#1-overview)
2. [Architecture](#2-architecture)
3. [Configuration](#3-configuration)
4. [CLI Commands](#4-cli-commands)
5. [Data Model & Storage](#5-data-model--storage) (Storage Provider Abstraction + Hybrid Columns/JSON)
6. [VCS Provider Abstraction](#6-vcs-provider-abstraction)
7. [Data Fetching](#7-data-fetching)
8. [Metrics Engine](#8-metrics-engine)
9. [Output & Formatting](#9-output--formatting)
10. [Hooks System](#10-hooks-system)
11. [Date Range Handling](#11-date-range-handling)
12. [Agent & Tool Integration](#12-agent--tool-integration)
13. [Error Handling](#13-error-handling)
14. [Open Source Considerations](#14-open-source-considerations)
15. [Future Roadmap](#15-future-roadmap)
16. [Appendix: Migration from pr-metrics-cli](#appendix-migration-from-pr-metrics-cli)

---

## 1. Overview

### 1.1 Purpose

prx is an enterprise-grade CLI tool that:

- Fetches pull request data from one or more GitHub instances (including GitHub Enterprise)
- Stores data locally in SQLite for efficient querying and historical analysis
- Calculates developer productivity metrics, PR lifecycle timings, and team-level reports
- Outputs structured data (JSON, tables, markdown) consumable by humans, CI pipelines, and AI agents
- Supports a hooks system for extending output processing (e.g., Slack notifications, AI-generated summaries)

### 1.2 Design Principles

1. **Offline-first**: Fetch once, analyze many times. All analysis runs against local SQLite data.
2. **Incremental by default**: Only fetch new/updated PRs on subsequent runs.
3. **Enterprise-ready**: Multi-instance GitHub support, team-based reporting, configurable tokens.
4. **Agent-friendly**: JSON output mode and SQLite database designed for consumption by AI agents and external tools.
5. **Extensible**: Hooks system allows post-processing without modifying the core tool.
6. **Simple defaults**: Works out of the box with minimal configuration. Advanced features opt-in.
7. **CI-friendly**: Non-interactive in non-TTY mode. Exit codes for scripting. JSON output for parsing.

### 1.3 Distribution

Single statically-linked binary. No runtime dependencies. Distributed via:
- GitHub Releases (prebuilt binaries for linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64)
- Homebrew tap
- Go install (`go install github.com/<org>/prx@latest`)

---

## 2. Architecture

### 2.1 Component Overview

```
                    ┌──────────────┐
                    │   CLI Layer   │  (command parsing, flags, TTY detection)
                    └──────┬───────┘
                           │
              ┌────────────┼────────────┐
              │            │            │
     ┌────────▼───┐  ┌────▼─────┐  ┌──▼──────────┐
     │  Fetcher    │  │ Analyzer │  │  Reporter    │
     │  (VCS API)  │  │ (Metrics)│  │  (Output)    │
     └────────┬───┘  └────┬─────┘  └──┬──────────┘
              │            │            │
              └────────────┼────────────┘
                           │
                    ┌──────▼───────┐
                    │  Store Layer  │  (storage provider abstraction)
                    └──────┬───────┘
                           │
                    ┌──────▼───────┐
                    │   SQLite      │  (default storage provider)
                    └──────┬───────┘
                           │
                    ┌──────▼───────┐
                    │  Hooks Engine │  (post-processing)
                    └──────────────┘
```

### 2.2 Module Boundaries

| Module | Responsibility |
|--------|---------------|
| `cli` | Command definitions, flag parsing, TTY detection, interactive prompts |
| `config` | YAML config loading, validation, defaults, environment variable resolution |
| `provider` | VCS provider interface + GitHub implementation |
| `store` | **Storage provider interface** — abstract CRUD, query, and migration operations |
| `store/sqlite` | **SQLite implementation** of the store interface (default provider) |
| `metrics` | All metric calculations (timing, volume, developer, team) |
| `report` | Output formatting (table, JSON, markdown) |
| `hooks` | Hook discovery, execution, stdin/stdout piping |

### 2.3 Data Flow

```
1. FETCH:   GitHub API  →  VCS Provider  →  Normalizer  →  SQLite
2. ANALYZE: SQLite  →  Query/Filter  →  Metrics Engine  →  Structured Result
3. REPORT:  Structured Result  →  Formatter  →  stdout / file
4. HOOKS:   Structured Result (JSON)  →  stdin of hook commands
```

---

## 3. Configuration

### 3.1 Config File Location

Resolution order (first found wins):
1. `--config <path>` CLI flag
2. `./prx.yaml` (current directory)
3. `$PRX_CONFIG` environment variable
4. `~/.config/prx/config.yaml`

### 3.2 Full Configuration Schema

```yaml
# prx.yaml

# ─── GitHub Instances ────────────────────────────────────────────────
# Multiple instances supported. Each has a unique name.
instances:
  # Name used to reference this instance in team/repo definitions
  github-com:
    type: github                          # Provider type (only "github" in v1)
    base_url: "https://api.github.com"    # API base URL
    # Token resolution: tries sources in order, uses first found
    token:
      env: GITHUB_TOKEN                   # Environment variable name
      # Future: file, vault, keychain, etc.

  enterprise:
    type: github
    base_url: "https://github.mycompany.com/api/v3"
    token:
      env: GHE_TOKEN
    # Optional: skip TLS verification (self-signed certs)
    tls_skip_verify: false

# ─── Teams ───────────────────────────────────────────────────────────
# Map team names to repositories. Repos reference instances by name.
teams:
  platform:
    display_name: "Platform Engineering"
    repos:
      - instance: enterprise
        repo: "org/platform-core"
      - instance: enterprise
        repo: "org/platform-infra"
      - instance: github-com
        repo: "company/oss-sdk"

  payments:
    display_name: "Payments Team"
    repos:
      - instance: enterprise
        repo: "org/payment-service"
      - instance: enterprise
        repo: "org/payment-gateway"

# ─── Standalone Repos ───────────────────────────────────────────────
# Repos not assigned to any team (can still be analyzed individually)
repos:
  - instance: github-com
    repo: "owner/standalone-repo"

# ─── Fetch Settings ─────────────────────────────────────────────────
fetch:
  states: ["closed", "open"]              # PR states to fetch: open, closed, all
  per_page: 100                           # Results per API page (max 100)
  max_retries: 3                          # Retry count on transient failures
  rate_limit_buffer: 100                  # Warn when remaining requests below this

# ─── Default Date Range ─────────────────────────────────────────────
# Used when no --start/--end/--preset is given on CLI
date_range:
  preset: "last-30d"                      # Default preset (see §11)
  # OR absolute:
  # start: "2026-01-01"
  # end: "2026-03-31"

# ─── Output Settings ────────────────────────────────────────────────
output:
  format: "table"                         # Default: table. Options: table, json, markdown, all
  directory: "./reports"                   # Directory for markdown file output
  timezone: "America/Toronto"             # Timezone for date display

# ─── Test File Patterns ─────────────────────────────────────────────
# Regex patterns to classify files as test vs production code
test_patterns:
  - "/__tests__/"
  - "/test/"
  - "/tests/"
  - "\\.test\\.(ts|js|tsx|jsx)$"
  - "\\.spec\\.(ts|js|tsx|jsx)$"
  - "Test\\.java$"
  - "_test\\.go$"
  - "/src/test/"

# ─── Hooks ───────────────────────────────────────────────────────────
hooks:
  post-analyze:
    - name: "slack-notify"
      command: "python3 scripts/slack_notify.py"
      # Hook receives JSON on stdin, can use env vars
      env:
        SLACK_WEBHOOK: "${SLACK_WEBHOOK_URL}"
    - name: "ai-summary"
      command: "node scripts/generate_summary.js"

  post-fetch:
    - name: "log-fetch"
      command: "echo 'Fetch complete' >> /var/log/prx.log"

# ─── Storage ─────────────────────────────────────────────────────────
# Storage provider configuration. SQLite is the default and only v1 provider.
# The storage layer is abstracted so alternative backends (PostgreSQL, etc.)
# can be added in the future without changing the rest of the codebase.
storage:
  provider: "sqlite"                          # Storage provider (only "sqlite" in v1)
  sqlite:
    path: "~/.config/prx/prx.db"  # SQLite file location
    # Default: ~/.config/prx/prx.db
    # Can be overridden for project-local DBs: "./prx.db"
```

### 3.3 Minimal Config (Quick Start)

```yaml
instances:
  default:
    type: github
    base_url: "https://api.github.com"
    token:
      env: GITHUB_TOKEN

repos:
  - instance: default
    repo: "owner/repo"
```

### 3.4 Environment Variable Substitution

Any string value in the config can reference environment variables using `${VAR_NAME}` syntax. This is resolved at config load time. Missing variables produce a clear error.

### 3.5 Config Validation

On load, the tool validates:
- All referenced instances exist
- All repos follow `owner/repo` format
- Token env vars are set (warn, don't fail — token might not be needed for all commands)
- Date formats are valid
- Test patterns compile as valid regex
- No duplicate team names or orphan instance references

---

## 4. CLI Commands

### 4.1 Global Flags

```
prx [command] [flags]

Global Flags:
  --config <path>       Path to config file
  --db <path>           Override database path
  --format <type>       Output format: table (default), json, markdown, all
  --quiet               Suppress non-essential output
  --verbose             Verbose/debug output
  --no-color            Disable colored output
  --version             Print version and exit
  --help                Print help
```

### 4.2 `prx init`

Generate a starter config file.

```
prx init [flags]

Flags:
  --path <path>         Where to create config (default: ./prx.yaml)
  --minimal             Generate minimal config (just instance + repos)
```

**Behavior:**
- Interactive in TTY mode: prompts for GitHub URL, token env var, repos
- Non-interactive: generates commented template
- Warns before overwriting existing file

### 4.3 `prx fetch`

Fetch PR data from GitHub and store in SQLite.

```
prx fetch [flags]

Flags:
  --team <name>         Fetch for a specific team's repos
  --repo <owner/repo>   Fetch for specific repo(s) (repeatable)
  --instance <name>     Limit to repos from this instance
  --full                Re-fetch all data (ignore incremental state)
  --dry-run             Show what would be fetched without making API calls
  --since <date>        Only fetch PRs updated after this date
  --states <list>       PR states: open,closed,all (overrides config)
```

**Behavior:**
- Default: incremental fetch — only PRs updated since last fetch timestamp
- `--full`: clears and re-fetches all data for targeted repos
- Displays progress: repo name, PR count, rate limit remaining
- Fetches for each PR: base PR data, branch comparison (commit history, file changes), timeline events (for draft PR detection)
- Stores fetch timestamp per repo for incremental tracking
- On rate limit exhaustion: pauses with countdown timer, resumes automatically
- Runs `post-fetch` hooks on completion

**Scope resolution (what repos to fetch):**
1. If `--repo` specified: those repos only
2. If `--team` specified: that team's repos
3. If neither: all repos (teams + standalone) from config

### 4.4 `prx analyze`

Calculate metrics from stored data and output results.

```
prx analyze [flags]

Flags:
  --team <name>         Analyze a specific team (repeatable for multi-team)
  --repo <owner/repo>   Analyze specific repo(s) (repeatable)
  --author <login>      Filter to specific author(s) (repeatable)
  --start <date>        Start date (YYYY-MM-DD)
  --end <date>          End date (YYYY-MM-DD)
  --preset <name>       Date preset (see §11)
  --group-by <field>    Group results: repo, team, author (default: none)
  --sort <field>        Sort developer table: prs-per-week, total-prs, avg-loc, avg-time-to-merge
  --top <n>             Show only top N developers
  --output <dir>        Output directory for markdown files
  --format <type>       table (default), json, markdown, all
```

**Behavior:**
- Reads from SQLite, applies date range + repo/team/author filters
- Calculates all metrics (see §8)
- Formats output per `--format`
- JSON output goes to stdout (pipeable)
- Markdown files written to output directory
- Table output goes to stderr (so JSON on stdout remains clean when piped)
- Runs `post-analyze` hooks with JSON result on stdin
- Exit code 0 on success, 1 on error

### 4.5 `prx summarize`

Generate business-friendly summaries of merged PRs.

```
prx summarize [flags]

Flags:
  --team <name>         Summarize for a team
  --repo <owner/repo>   Summarize specific repo(s) (repeatable)
  --start <date>        Start date
  --end <date>          End date
  --preset <name>       Date preset
  --format <type>       json (default), markdown, table
```

**Output (JSON):**
```json
{
  "period": { "start": "2026-03-01", "end": "2026-03-25" },
  "team": "platform",
  "repos": ["org/platform-core", "org/platform-infra"],
  "total_merged_prs": 42,
  "total_loc_changed": 8234,
  "developers": [
    {
      "login": "alice",
      "merged_prs": 15,
      "prs": [
        {
          "number": 234,
          "title": "Add retry logic to payment processor",
          "url": "https://github.com/org/repo/pull/234",
          "additions": 145,
          "deletions": 23,
          "files_changed": 6,
          "test_loc": 89,
          "prod_loc": 79,
          "merged_at": "2026-03-15T14:30:00Z"
        }
      ]
    }
  ]
}
```

This structured output is designed for AI agents to generate narrative status reports.

### 4.6 `prx status`

Show overview of fetched data.

```
prx status [flags]

Flags:
  --team <name>         Show status for team's repos only
```

**Output:**
```
Database: ~/.config/prx/prx.db (4.2 MB)

  Repository                          PRs    Last Fetched         Date Range
  ──────────────────────────────────  ─────  ───────────────────  ──────────────────
  enterprise:org/platform-core        342    2026-03-25 09:15     2025-01-15 → 2026-03-24
  enterprise:org/platform-infra       128    2026-03-25 09:15     2025-06-01 → 2026-03-23
  github-com:company/oss-sdk           56    2026-03-24 16:30     2025-09-01 → 2026-03-20

Teams:
  platform (3 repos, 526 PRs)
  payments (2 repos, 203 PRs)

Total: 5 repos, 729 PRs across 2 instances
```

### 4.7 `prx teams`

Manage and inspect team configuration.

```
prx teams                     # List all teams and their repos
prx teams show <name>         # Show detailed team info
```

### 4.8 `prx report`

Combined fetch + analyze in a single command. This is the primary command for most users.

```
prx report [flags]

Flags:
  (accepts ALL flags from both `fetch` and `analyze`)
  --skip-fetch          Skip the fetch step (analyze existing data only)
  --fetch-only          Stop after fetching (don't analyze)
```

**Behavior:**
1. Runs `fetch` (with applicable fetch flags: `--team`, `--repo`, `--instance`, `--full`, `--states`)
2. Runs `analyze` (with applicable analyze flags: `--start`, `--end`, `--preset`, `--group-by`, `--format`, etc.)
3. Runs hooks (both `post-fetch` and `post-analyze` in sequence)

This is the "do the right thing" command. Most users should only need:
```bash
prx report --team platform --preset last-week
prx report --repo owner/repo --start 2026-01-01 --end 2026-03-31
prx report --team payments --preset this-month --format json
```

### 4.9 `prx export`

Export data from SQLite for debugging or portability.

```
prx export [flags]

Flags:
  --repo <owner/repo>   Export specific repo(s)
  --team <name>         Export team's repos
  --start <date>        Filter by date
  --end <date>          Filter by date
  --format <type>       json (default), csv
  --output <path>       Output file (default: stdout)
```

### 4.9 `prx db`

Direct database access for power users and agents.

```
prx db query <sql>            # Run a read-only SQL query against the DB
prx db path                   # Print the database file path
prx db stats                  # Show database size, table row counts
prx db raw <repo> <pr#>       # Dump raw JSON blob for a specific PR (debugging)
```

This is critical for agent integration — an AI agent can run arbitrary read-only SQL against the metrics database. The `raw` subcommand lets you inspect the full API response stored in the JSON blob.

Example: querying raw data for fields not yet promoted to columns:
```bash
prx db query "SELECT number, json_extract(raw_data, '$.labels') FROM pull_requests WHERE repo_id=1"
```

---

## 5. Data Model & Storage

### 5.1 Storage Provider Abstraction

Storage is accessed through an abstract interface, allowing the backend to be swapped without changing any upstream code. SQLite is the default (and only v1) provider.

```
Interface: StorageProvider
  // Lifecycle
  - Open(config) → error
  - Close() → error
  - Migrate() → error                          // Run schema migrations

  // Write operations
  - UpsertInstance(instance) → id, error
  - UpsertRepository(repo) → id, error
  - UpsertPullRequest(pr) → id, error          // Insert or update by (repo_id, number)
  - UpsertBranchInfo(branchInfo) → error
  - UpsertFileChanges(prId, files[]) → error   // Replace all file changes for a PR
  - UpdateFetchMetadata(repoId, metadata) → error

  // Read operations
  - GetPullRequest(repoId, number) → PR, error
  - ListPullRequests(filters) → []PR, error    // Filters: repo, team, author, date range, state
  - GetFetchMetadata(repoId) → metadata, error
  - ListRepositories() → []Repository, error
  - GetPRState(repoId, number) → (state, updatedAt), error  // Lightweight check for sync logic

  // Query (power users / agents)
  - RawQuery(sql) → rows, error               // Read-only arbitrary SQL (SQLite provider only)
  - Stats() → StorageStats, error             // Row counts, DB size

  // Bulk
  - BeginTx() → Transaction, error
  - WithinTx(fn) → error                      // Execute fn within a transaction
```

**Why an abstraction?** Future providers could include:
- PostgreSQL (for shared/team-wide databases)
- Cloud-hosted SQLite (Turso, LiteFS)
- Remote API backend (for a future server mode)

Configuration:
```yaml
storage:
  provider: "sqlite"           # Only "sqlite" in v1
  sqlite:
    path: "~/.config/prx/prx.db"
```

### 5.2 Hybrid Storage Model: Columns + JSON Blob

Every data table uses a **hybrid approach**: frequently-queried fields are stored as indexed columns, while the full raw API response is preserved as a JSON blob. This gives us:

- **Fast queries** on columns we know we need (dates, state, author, LOC)
- **Future-proofing**: any data from the API we didn't anticipate needing is preserved in the blob
- **Zero data loss**: the raw response is always available for re-processing
- **Easy evolution**: when a new metric needs a field from the blob, we add a column via migration and backfill from JSON

### 5.3 SQLite Schema

```sql
-- ─── Instances ──────────────────────────────────────────────────────
CREATE TABLE instances (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT NOT NULL UNIQUE,              -- Config name (e.g., "enterprise")
    type        TEXT NOT NULL DEFAULT 'github',    -- Provider type
    base_url    TEXT NOT NULL
);

-- ─── Repositories ───────────────────────────────────────────────────
CREATE TABLE repositories (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    instance_id INTEGER NOT NULL REFERENCES instances(id),
    owner       TEXT NOT NULL,
    name        TEXT NOT NULL,
    full_name   TEXT NOT NULL,                     -- "owner/name"
    UNIQUE(instance_id, full_name)
);

-- ─── Teams ──────────────────────────────────────────────────────────
CREATE TABLE teams (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    name         TEXT NOT NULL UNIQUE,
    display_name TEXT
);

CREATE TABLE team_repos (
    team_id     INTEGER NOT NULL REFERENCES teams(id),
    repo_id     INTEGER NOT NULL REFERENCES repositories(id),
    PRIMARY KEY (team_id, repo_id)
);

-- ─── Pull Requests ──────────────────────────────────────────────────
-- Indexed columns for fields we query/filter/sort on.
-- raw_data JSON blob preserves the full API response for future use.
CREATE TABLE pull_requests (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    repo_id             INTEGER NOT NULL REFERENCES repositories(id),
    number              INTEGER NOT NULL,
    title               TEXT NOT NULL,
    state               TEXT NOT NULL,              -- open, closed, merged
    author              TEXT NOT NULL,               -- GitHub login
    url                 TEXT NOT NULL,               -- HTML URL
    created_at          TEXT NOT NULL,               -- ISO 8601
    updated_at          TEXT NOT NULL,
    merged_at           TEXT,                        -- NULL if not merged
    closed_at           TEXT,                        -- NULL if still open
    is_draft            INTEGER NOT NULL DEFAULT 0,  -- Boolean
    ready_for_review_at TEXT,                        -- When draft became ready
    additions           INTEGER NOT NULL DEFAULT 0,
    deletions           INTEGER NOT NULL DEFAULT 0,
    changed_files       INTEGER NOT NULL DEFAULT 0,
    base_branch         TEXT,                        -- e.g., "main"
    head_branch         TEXT,                        -- e.g., "feature/xyz"
    body                TEXT,                        -- PR description
    raw_data            TEXT,                        -- Full API response as JSON blob
    UNIQUE(repo_id, number)
);

-- ─── Branch Info ────────────────────────────────────────────────────
CREATE TABLE branch_info (
    pr_id               INTEGER PRIMARY KEY REFERENCES pull_requests(id),
    merge_base_sha      TEXT,
    first_commit_date   TEXT,                       -- ISO 8601, branch creation proxy
    commits_count       INTEGER NOT NULL DEFAULT 0,
    total_additions     INTEGER NOT NULL DEFAULT 0,
    total_deletions     INTEGER NOT NULL DEFAULT 0,
    raw_data            TEXT                        -- Full compare API response as JSON blob
);

-- ─── File Changes ───────────────────────────────────────────────────
CREATE TABLE file_changes (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    pr_id       INTEGER NOT NULL REFERENCES pull_requests(id),
    filename    TEXT NOT NULL,
    additions   INTEGER NOT NULL DEFAULT 0,
    deletions   INTEGER NOT NULL DEFAULT 0,
    is_test     INTEGER NOT NULL DEFAULT 0          -- Classified by test patterns
);

CREATE INDEX idx_file_changes_pr_id ON file_changes(pr_id);

-- ─── Timeline Events ──────────────────────────────────────────────
-- Store all timeline events as JSON for future analysis.
-- Extracted fields (ready_for_review_at) live on pull_requests table.
CREATE TABLE timeline_events (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    pr_id       INTEGER NOT NULL REFERENCES pull_requests(id),
    event_type  TEXT NOT NULL,
    created_at  TEXT,
    actor       TEXT,
    raw_data    TEXT NOT NULL                       -- Full event as JSON blob
);

CREATE INDEX idx_timeline_pr_id ON timeline_events(pr_id);

-- ─── Fetch Metadata ────────────────────────────────────────────────
CREATE TABLE fetch_metadata (
    repo_id         INTEGER PRIMARY KEY REFERENCES repositories(id),
    last_fetch_at   TEXT NOT NULL,                  -- ISO 8601
    last_updated_at TEXT,                           -- Most recent PR updated_at seen
    pr_count        INTEGER NOT NULL DEFAULT 0
);

-- ─── Useful Indexes ────────────────────────────────────────────────
CREATE INDEX idx_prs_repo_state ON pull_requests(repo_id, state);
CREATE INDEX idx_prs_author ON pull_requests(author);
CREATE INDEX idx_prs_merged_at ON pull_requests(merged_at);
CREATE INDEX idx_prs_created_at ON pull_requests(created_at);
CREATE INDEX idx_prs_updated_at ON pull_requests(updated_at);

-- ─── Schema Version ────────────────────────────────────────────────
CREATE TABLE schema_version (
    version     INTEGER NOT NULL,
    applied_at  TEXT NOT NULL
);
```

### 5.4 JSON Blob Usage Guidelines

- **Write**: Always store the full raw API response in `raw_data` at fetch time
- **Read (normal operations)**: Always use indexed columns for queries — never parse `raw_data` in hot paths
- **Read (ad-hoc / agent)**: `raw_data` available via `prx db query` for SQL with `json_extract()`
- **Migration path**: When a field from `raw_data` is needed for metrics, add a column + migration that backfills from the JSON blob. Example:
  ```sql
  -- Migration 003: Add labels column, backfill from raw_data
  ALTER TABLE pull_requests ADD COLUMN labels TEXT;
  UPDATE pull_requests SET labels = json_extract(raw_data, '$.labels');
  ```

### 5.5 Schema Migrations

- Schema version tracked in `schema_version` table
- Migrations are numbered sequentially (001, 002, ...)
- Migrations run automatically on startup if DB is behind
- Forward-only migrations (no rollback in v1)
- Each migration is atomic (wrapped in a transaction)
- Backfill migrations can populate new columns from `raw_data` JSON blobs

### 5.6 Database Location

Default: `~/.config/prx/prx.db`

Override via:
1. `--db <path>` CLI flag
2. `storage.sqlite.path` in config
3. `$PRX_DB` environment variable

### 5.7 Concurrency

SQLite in WAL mode for concurrent read access. Single-writer model. Fetch operations acquire a write lock for batch inserts (using transactions for performance).

---

## 6. VCS Provider Abstraction

### 6.1 Provider Interface

```
Interface: VCSProvider
  - ListPullRequests(repo, options) → []PullRequest
  - GetPullRequest(repo, number) → PullRequest
  - GetBranchComparison(repo, base, head) → BranchComparison
  - GetTimelineEvents(repo, prNumber) → []TimelineEvent
  - GetRateLimit() → RateLimit
  - Name() → string
```

### 6.2 Normalized Data Types

All providers must normalize their data into these common types:

```
PullRequest:
  number, title, state (open/closed/merged), author,
  url, created_at, updated_at, merged_at, closed_at,
  is_draft, additions, deletions, changed_files,
  base_branch, head_branch, body

BranchComparison:
  merge_base_sha, first_commit_date, commits_count,
  total_additions, total_deletions, files[]

FileChange:
  filename, additions, deletions

TimelineEvent:
  event_type, created_at, actor, data

RateLimit:
  limit, remaining, reset_at
```

### 6.3 GitHub Implementation (v1)

- Uses GitHub REST API via HTTP client (no SDK dependency for portability)
- Endpoints used:
  - `GET /repos/{owner}/{repo}/pulls` — list PRs
  - `GET /repos/{owner}/{repo}/pulls/{number}` — PR detail (additions/deletions)
  - `GET /repos/{owner}/{repo}/compare/{base}...{head}` — branch comparison
  - `GET /repos/{owner}/{repo}/issues/{number}/timeline` — timeline events
  - `GET /rate_limit` — rate limit status
- Authentication: `Authorization: Bearer <token>` header
- Pagination: Link header parsing, configurable per_page
- Rate limiting: check before requests, pause when near limit
- Retry: exponential backoff on 5xx and network errors, max 3 retries
- No retry on 401, 403, 404

### 6.4 Adding New Providers (Future)

To add GitLab or Bitbucket support:
1. Implement the `VCSProvider` interface
2. Add a new `type` value (e.g., `gitlab`) in instance config
3. Register the provider in the provider factory
4. All downstream code (store, metrics, reports) works unchanged

---

## 7. Data Fetching

### 7.1 Smart Sync Strategy

The fetcher is designed to **minimize API calls** while keeping data fresh. The core principle: **closed/merged PRs are immutable; open PRs may change.**

#### Decision Matrix (per PR encountered from API)

| PR State (API) | Exists in DB? | DB State | Action |
|----------------|---------------|----------|--------|
| open | No | — | **Fetch full** (PR + branch + timeline + files) |
| open | Yes | open | **Re-fetch** (PR may have new commits, reviews, state changes) |
| open | Yes | closed/merged | **Re-fetch** (reopened PR — rare but possible) |
| closed/merged | No | — | **Fetch full** |
| closed/merged | Yes | closed/merged | **Skip** (immutable — no API calls needed) |
| closed/merged | Yes | open | **Re-fetch** (PR was closed/merged since last fetch) |

#### Why This Matters

A repository with 500 PRs where 480 are merged and 20 are open:
- **Naive approach**: re-fetch all 500 = ~1,500 API calls (PR + branch + timeline each)
- **Smart sync**: fetch 20 open + any new = ~60 API calls (97% reduction)

### 7.2 Incremental Fetch Algorithm

```
1. Read fetch_metadata.last_updated_at for the repo
2. Fetch PR list from API, sorted by updated_at descending, filtered by since=last_updated_at
3. For each PR from API:
   a. Check local DB: SELECT state, updated_at FROM pull_requests WHERE repo_id=? AND number=?
   b. Apply decision matrix (§7.1):
      - If SKIP: do nothing (log in verbose mode)
      - If FETCH FULL: fetch PR detail + branch comparison + timeline + file changes
      - If RE-FETCH: fetch PR detail + timeline (branch info only if new commits detected)
   c. Upsert all data into DB within transaction
4. For each page: if ALL PRs on page have updated_at before our last fetch, stop pagination
5. Update fetch_metadata with new timestamp
6. Report: "Fetched N new, M updated, S skipped PRs for owner/repo"
```

### 7.3 Full Refresh

`prx fetch --full` bypasses the smart sync and re-fetches everything. Use when:
- Test patterns changed (need to re-classify files)
- Suspect data corruption
- First fetch after upgrading prx (to populate new `raw_data` blobs)

### 7.4 Fetch Pipeline (per PR)

```
For a FULL fetch of a single PR:
1. GET /repos/{owner}/{repo}/pulls/{number}         → PR detail (additions, deletions, etc.)
2. GET /repos/{owner}/{repo}/compare/{base}...{head} → Branch comparison (commits, files)
3. GET /repos/{owner}/{repo}/issues/{number}/timeline → Timeline events (draft, reviews)
4. Classify files as test/prod using configured patterns
5. Store everything in DB:
   - pull_requests row (columns + raw_data JSON blob)
   - branch_info row (columns + raw_data JSON blob)
   - file_changes rows (one per file, with is_test flag)
   - timeline_events rows (one per event, with raw_data JSON blob)
   - Extract ready_for_review_at from timeline → update pull_requests

For a RE-FETCH of an existing PR:
1. GET /repos/{owner}/{repo}/pulls/{number}         → PR detail
2. Compare commits_count with stored branch_info:
   - If changed: re-fetch branch comparison + files
   - If same: skip (saves 1 API call)
3. GET /repos/{owner}/{repo}/issues/{number}/timeline → Timeline (always re-fetch for open PRs)
4. Upsert all changed data
```

### 7.5 Rate Limit Management

- Check remaining quota before each repo
- If remaining < `rate_limit_buffer` (default 100): warn user, ask to continue (in TTY) or abort (non-TTY)
- If rate limited (403 with rate limit headers): sleep until reset time + 1 second, auto-retry
- Display remaining quota in verbose mode
- Log API calls saved by smart sync in verbose mode

### 7.6 Batch Operations

- Use SQLite transactions for batch inserts (1 transaction per repo)
- Prepare statements once, execute many times
- Target: 100+ PR inserts per second

### 7.7 Fetch Progress Display

```
Fetching org/platform-core...
  PRs: ████████████████████░░░░ 156/200  (skipped: 134, new: 12, updated: 10)
  API calls: 47/5000 remaining
  ETA: ~30s
```

---

## 8. Metrics Engine

### 8.1 Timing Metrics

All timing metrics calculated from stored data. Times are in hours (stored) and formatted as human-readable durations for display.

| Metric | Calculation | Description |
|--------|------------|-------------|
| **Time to Open** | `pr.created_at - branch_info.first_commit_date` | Coding time: how long between first commit and opening a PR |
| **Draft Time** | `pr.ready_for_review_at - pr.created_at` | Time spent in draft state (0 if never draft) |
| **Time to Merge** | `pr.merged_at - review_start_time` | Review time: from ready-for-review to merge |
| **Total Time** | `pr.merged_at - branch_info.first_commit_date` | Full lifecycle from first commit to merge |

Where `review_start_time` = `ready_for_review_at` if PR was draft, otherwise `created_at`.

### 8.2 Volume Metrics

| Metric | Description |
|--------|-------------|
| Total PRs | All PRs in range (merged + closed + open) |
| Merged PRs | Successfully merged |
| Closed PRs | Closed without merging |
| Open PRs | Still open at end of range |
| Unique Authors | Distinct PR authors |
| Avg PRs/Author | Total PRs / unique authors |

### 8.3 Size Metrics

| Metric | Description |
|--------|-------------|
| Total LOC Changed | Additions + deletions across all PRs |
| Test LOC | LOC in files matching test patterns |
| Production LOC | LOC in files NOT matching test patterns |
| Avg LOC/PR | Total LOC / PR count |
| Median LOC/PR | Median of per-PR LOC |

### 8.4 Developer Metrics

Per-developer breakdown (for each unique `author`):

| Metric | Description |
|--------|-------------|
| Merged PRs | Count of merged PRs by this author |
| PRs/Week | Merged PRs / weeks in analysis period |
| Total/Avg/Median LOC | Size metrics scoped to this author |
| Test LOC / Prod LOC | Separated by file classification |
| Avg Time to Open | Average coding time for this author |
| Avg Draft Time | Average draft duration |
| Avg Time to Merge | Average review time |
| Avg Total Time | Average full lifecycle |

### 8.5 Team Metrics

When analyzing by team (`--team` or `--group-by team`):
- All above metrics aggregated at team level
- Per-repo breakdown within team
- Cross-team comparison when multiple teams analyzed

### 8.6 Statistical Functions

- **Average**: sum / count
- **Median**: middle value (average of two middle for even count)
- **Sum**: total of values
- **Min weeks**: 0.14 (1 day) minimum to avoid division by zero in PRs/week

### 8.7 File Classification

Files are classified as test or production using configurable regex patterns (see config `test_patterns`).

Default patterns cover:
- JavaScript/TypeScript: `__tests__/`, `.test.ts`, `.spec.tsx`
- Java: `Test.java`, `src/test/`
- Go: `_test.go`
- Generic: `/test/`, `/tests/`, `/spec/`

Classification happens at fetch time and is stored in `file_changes.is_test`. Re-classification requires re-fetch (`--full`).

---

## 9. Output & Formatting

### 9.1 Output Formats

| Format | Flag | Destination | Use Case |
|--------|------|-------------|----------|
| `table` | `--format table` | stderr | Human reading in terminal |
| `json` | `--format json` | stdout | Agent consumption, piping, scripting |
| `markdown` | `--format markdown` | file | Reports, wikis, documentation |
| `all` | `--format all` | stderr + stdout + file | Everything at once |

**Important**: Table output goes to stderr so that `prx analyze --format json` can be piped cleanly. When format is `table` only, tables go to stdout.

### 9.2 Table Output

#### Summary Table
```
  Metric                  Value
  ──────────────────────  ──────────
  Total PRs               42
  Merged PRs              38
  Closed (not merged)     2
  Open                    2
  Unique Authors          6
  Total LOC Changed       8,234
    Test LOC              2,891
    Production LOC        5,343
  Avg LOC/PR              217
  Median LOC/PR           142
  Avg Time to Merge       1 day 4 hours
  Median Time to Merge    18 hours
```

#### Developer Table (sorted by PRs/Week descending)
```
  Developer   Merged  PRs/Wk  Avg LOC  Avg Test  Avg Prod  Avg→Open   Avg→Merge  Avg Total
  ──────────  ──────  ──────  ───────  ────────  ────────  ─────────  ─────────  ─────────
  alice       15      3.75    245      89        156       4 hours    18 hours   1 day 2h
  bob         12      3.00    198      45        153       6 hours    1 day      1 day 12h
```

#### Slowest PRs Table (top 10)
```
  PR#    Author   Title                       →Open     Draft    →Merge    Total     LOC
  ─────  ───────  ──────────────────────────  ────────  ───────  ────────  ────────  ─────
  #234   alice    Add retry logic to payme…   2 hours   0        5 days    5 days    168
  #189   bob      Refactor auth middleware…   1 day     3 hours  4 days    5 days    423
```

### 9.3 JSON Output

The JSON output is the canonical structured representation. It is designed for:
- AI agents to parse and reason about
- External tools to consume
- Piping to hooks

```json
{
  "meta": {
    "tool": "prx",
    "version": "1.0.0",
    "generated_at": "2026-03-25T10:30:00Z",
    "date_range": { "start": "2026-03-01", "end": "2026-03-25" },
    "repos": ["org/platform-core", "org/platform-infra"],
    "team": "platform",
    "filters": { "authors": null, "states": ["merged"] }
  },
  "summary": {
    "total_prs": 42,
    "merged_prs": 38,
    "closed_prs": 2,
    "open_prs": 2,
    "unique_authors": 6,
    "loc": {
      "total": 8234,
      "test": 2891,
      "production": 5343,
      "avg_per_pr": 217,
      "median_per_pr": 142
    },
    "timing": {
      "avg_time_to_open_hours": 8.5,
      "avg_time_to_merge_hours": 28.3,
      "avg_total_time_hours": 36.8,
      "median_time_to_merge_hours": 18.0
    }
  },
  "developers": [
    {
      "login": "alice",
      "merged_prs": 15,
      "prs_per_week": 3.75,
      "loc": { "avg_total": 245, "avg_test": 89, "avg_production": 156 },
      "timing": {
        "avg_time_to_open_hours": 4.0,
        "avg_draft_time_hours": 0,
        "avg_time_to_merge_hours": 18.0,
        "avg_total_time_hours": 26.0
      }
    }
  ],
  "slowest_prs": [
    {
      "repo": "org/platform-core",
      "number": 234,
      "author": "alice",
      "title": "Add retry logic to payment processor",
      "url": "https://github.com/org/repo/pull/234",
      "time_to_open_hours": 2.0,
      "draft_time_hours": 0,
      "time_to_merge_hours": 120.0,
      "total_time_hours": 122.0,
      "loc": { "total": 168, "test": 89, "production": 79 }
    }
  ],
  "repo_breakdown": [
    {
      "repo": "org/platform-core",
      "merged_prs": 28,
      "unique_authors": 5,
      "loc": { "total": 6100, "test": 2100, "production": 4000 }
    }
  ]
}
```

### 9.4 Markdown Report

Generated as a file in the output directory.

**Filename convention:**
- Single repo: `{owner}-{repo}-{YYYY-MM-DD}.md`
- Team: `{team-name}-{YYYY-MM-DD}.md`
- Multi-repo: `multi-repo-{N}-repos-{YYYY-MM-DD}.md`

**Content sections:**
1. Title with repo/team name
2. Date range and generation timestamp
3. Summary metrics table
4. Size metrics table
5. Timing metrics table
6. Developer metrics table
7. Repository breakdown (if multi-repo)
8. Top 20 slowest PRs

---

## 10. Hooks System

### 10.1 Overview

Hooks are user-configured shell commands that run after specific events. They receive structured data on stdin and can perform any action: send notifications, call AI APIs, write to dashboards, etc.

### 10.2 Hook Events

| Event | Trigger | stdin Data |
|-------|---------|-----------|
| `post-fetch` | After a successful fetch | JSON: `{ repos: [...], new_prs: N, updated_prs: M }` |
| `post-analyze` | After analysis completes | Full JSON output (same as `--format json`) |

### 10.3 Hook Configuration

```yaml
hooks:
  post-analyze:
    - name: "slack-notify"                # Human-readable name (for logs)
      command: "python3 ./hooks/slack.py" # Shell command to execute
      timeout: 30                         # Timeout in seconds (default: 30)
      env:                                # Extra environment variables
        SLACK_CHANNEL: "#engineering"
      on_error: "warn"                    # warn (default) or fail
```

### 10.4 Hook Execution

1. Hook commands run sequentially in definition order
2. JSON data piped to stdin
3. stdout/stderr from hooks displayed with `[hook:name]` prefix
4. Non-zero exit code: log warning (or fail if `on_error: fail`)
5. Timeout: kill the process, log warning
6. Hooks inherit the tool's environment variables plus any extras defined in config
7. Working directory: the directory where prx was invoked

### 10.5 Built-in Hook Helpers

The tool ships with example hook scripts in the repository (not bundled in the binary):
- `hooks/examples/slack_webhook.py` — Post summary to Slack
- `hooks/examples/ai_summary.sh` — Pipe to an LLM CLI for narrative generation
- `hooks/examples/csv_export.py` — Convert JSON to CSV
- `hooks/examples/html_report.py` — Generate HTML dashboard

---

## 11. Date Range Handling

### 11.1 Resolution Priority

1. `--start` and `--end` CLI flags (explicit dates)
2. `--preset` CLI flag
3. `date_range` in config file
4. Default: `last-30d`

### 11.2 Named Presets

| Preset | Meaning |
|--------|---------|
| `today` | Start of today → now |
| `yesterday` | Start of yesterday → end of yesterday |
| `this-week` | Monday of current week → now |
| `last-week` | Monday → Sunday of previous week |
| `this-month` | 1st of current month → now |
| `last-month` | 1st → last day of previous month |
| `this-quarter` | 1st of current quarter → now |
| `last-quarter` | Full previous quarter |
| `this-year` | Jan 1 of current year → now |

### 11.3 Rolling Windows

| Preset | Meaning |
|--------|---------|
| `last-7d` | 7 days ago → now |
| `last-14d` | 14 days ago → now |
| `last-30d` | 30 days ago → now |
| `last-60d` | 60 days ago → now |
| `last-90d` | 90 days ago → now |
| `last-Nd` | N days ago → now (any positive integer) |

### 11.4 Explicit Dates

```bash
prx analyze --start 2026-01-01 --end 2026-03-31
prx analyze --start 2026-01-01                    # end defaults to today
```

Date format: `YYYY-MM-DD` (always). Timezone from config or system default.

### 11.5 Date Filtering Behavior

- **Fetch**: filters by `updated_at` (to capture recently changed PRs)
- **Analyze**: filters by `created_at` within range (PR was opened in this period)
- Merged/closed dates are reported but not used for primary filtering
- PRs that span the boundary (opened before range, merged within) are included if created_at is in range

---

## 12. Agent & Tool Integration

### 12.1 Design for AI Agents

prx is designed to be used as a data source for AI agents. Key integration points:

1. **JSON output**: `prx analyze --format json` produces structured, parseable output
2. **SQL access**: `prx db query "SELECT ..."` allows agents to run arbitrary read-only queries
3. **Summarize command**: `prx summarize` outputs agent-optimized JSON with PR details
4. **SQLite file**: Agents can directly open the SQLite database file
5. **Hooks**: Agents can be invoked as post-processing hooks
6. **Exit codes**: 0 = success, 1 = error (for agent workflow orchestration)

### 12.2 Agent Workflow Examples

#### Example 1: Weekly Status Report via Claude
```bash
# Agent runs these commands:
prx fetch --team platform
prx summarize --team platform --preset last-week --format json | \
  claude "Generate a weekly engineering status report from this PR data. \
          Highlight key accomplishments, areas of concern, and trends."
```

#### Example 2: PR Velocity Dashboard
```bash
# Agent queries the database directly:
prx db query "
  SELECT author,
         COUNT(*) as merged_prs,
         AVG(julianday(merged_at) - julianday(created_at)) * 24 as avg_hours_to_merge
  FROM pull_requests
  WHERE merged_at IS NOT NULL
    AND merged_at > date('now', '-30 days')
  GROUP BY author
  ORDER BY merged_prs DESC
"
```

#### Example 3: Hook-based AI Summary
```yaml
# In prx.yaml:
hooks:
  post-analyze:
    - name: "ai-weekly-report"
      command: "claude --format text -p 'Summarize this engineering metrics data into a 3-paragraph executive summary'"
```

### 12.3 Structured Output Contract

The JSON output schema (§9.3) is a stable contract. Fields will not be removed in minor versions. New fields may be added. Agents should ignore unknown fields.

---

## 13. Error Handling

### 13.1 Error Categories

| Category | Behavior | Exit Code |
|----------|----------|-----------|
| Config errors | Print specific validation error, suggest fix | 1 |
| Auth errors (401) | Print message with token setup instructions | 1 |
| Permission errors (403) | Identify repo, suggest checking access | 1 |
| Repo not found (404) | Skip repo, warn, continue with others | 0 (with warnings) |
| Rate limited (429/403) | Auto-wait until reset, then retry | 0 |
| Network errors | Retry with exponential backoff (3 attempts) | 1 if all fail |
| Corrupt DB | Print error, suggest re-fetch | 1 |
| No data | Helpful message: "No data found. Run `prx fetch` first." | 1 |
| Hook failures | Warn by default, fail if `on_error: fail` | 0 or 1 |

### 13.2 TTY vs Non-TTY Behavior

| Feature | TTY (interactive) | Non-TTY (CI/pipe) |
|---------|-------------------|-------------------|
| Progress spinners | Shown | Hidden |
| Color | Enabled | Disabled |
| Prompts | Interactive | Auto-accept defaults or error |
| Rate limit warning | Prompt to continue | Continue automatically |
| Table output | Formatted with borders | Same (or use `--format json` in CI) |

### 13.3 Verbose Mode

`--verbose` enables:
- API request/response logging (URL, status, timing)
- Rate limit remaining after each request
- SQL queries being executed
- Hook stdin/stdout/stderr
- Config resolution details

---

## 14. Open Source Considerations

### 14.1 Repository Structure

```
prx/
├── cmd/                    # CLI entry points
│   └── prx/
│       └── main.go         # (or equivalent for chosen language)
├── internal/               # Private packages
│   ├── cli/                # Command definitions
│   ├── config/             # Config loading & validation
│   ├── provider/           # VCS provider interface + GitHub impl
│   │   ├── interface.go
│   │   └── github/
│   ├── store/              # SQLite operations
│   ├── metrics/            # Metric calculations
│   ├── report/             # Output formatting
│   └── hooks/              # Hook execution engine
├── hooks/                  # Example hook scripts
│   └── examples/
├── docs/                   # Documentation
│   ├── configuration.md
│   ├── metrics.md
│   └── hooks.md
├── prx.example.yaml  # Example config
├── README.md
├── LICENSE                 # MIT or Apache 2.0
├── CONTRIBUTING.md
└── .goreleaser.yaml        # Release automation
```

### 14.2 Documentation Requirements

- README with quick start (3-step setup)
- Configuration reference (all fields documented)
- Metrics glossary (what each metric means and how it's calculated)
- Hooks development guide
- Agent integration cookbook (examples with Claude, GPT, etc.)
- `--help` text for every command and flag

### 14.3 Release Process

- Semantic versioning (MAJOR.MINOR.PATCH)
- Prebuilt binaries for all major platforms via GoReleaser (or equivalent)
- Homebrew formula
- Changelog generated from conventional commits

---

## 15. Future Roadmap

Items explicitly deferred from v1:

### v1.1 — Alternative Storage Providers
- PostgreSQL storage provider (for shared/team-wide databases)
- Cloud SQLite (Turso, LiteFS) for distributed access

### v1.2 — Enhanced Review Metrics
- Reviewer tracking (who reviewed, how quickly)
- Time to first review
- Review rounds count
- Comment density
- Approval patterns

### v1.3 — Alerting & Thresholds
- Configurable thresholds in config (e.g., `warn_if_avg_merge_time > 72h`)
- CLI exit codes for threshold violations (CI gate use case)
- Threshold report section in output

### v1.4 — Additional VCS Providers
- GitLab implementation
- Bitbucket implementation
- Azure DevOps implementation

### v1.5 — Web Dashboard
- Optional lightweight HTTP server mode
- Real-time metrics dashboard
- Historical trend charts

### v2.0 — Advanced Features
- Sprint tracking (configurable sprint boundaries)
- DORA metrics (deployment frequency, lead time, MTTR, change failure rate)
- Custom metric definitions via config
- Webhook receiver for real-time PR event ingestion
- Multi-user access controls for server mode

---

## Appendix: Migration from pr-metrics-cli

### Feature Parity Checklist

| pr-metrics-cli Feature | prx Equivalent | Status |
|------------------------|---------------------|--------|
| `init` command | `prx init` | Parity |
| `fetch` command | `prx fetch` | Enhanced (multi-instance, teams) |
| `fetch --incremental` | Default behavior | Enhanced (smarter incremental) |
| `fetch --refetch` | `prx fetch --full` | Renamed |
| `fetch --dry-run` | `prx fetch --dry-run` | Parity |
| `status` command | `prx status` | Enhanced (teams, instances) |
| `analyze` command | `prx analyze` | Enhanced (teams, presets, grouping) |
| `summarize` command | `prx summarize` | Parity |
| TOML config | YAML config | Changed format |
| JSON file storage | SQLite | Changed storage |
| `--format table` | `--format table` | Parity |
| `--format json` | `--format json` | Enhanced (stable schema) |
| `--format markdown` | `--format markdown` | Parity |
| `--format both` | `--format all` | Renamed |
| Test file classification | Test file classification | Parity |
| Multi-repo analysis | Multi-repo + team analysis | Enhanced |
| GHE support | Multi-instance support | Enhanced |
| Date range (start/end) | Presets + rolling + explicit | Enhanced |
| Rate limit handling | Rate limit handling | Parity |

### New in prx

- `prx report` — combined fetch + analyze in one command
- Team-based organization and reporting
- Multiple GitHub instances in single config
- Storage provider abstraction (SQLite default, swappable)
- Hybrid columns + JSON blob storage (future-proof, zero data loss)
- Smart sync: skips re-fetching closed/merged PRs (massive API savings)
- SQLite storage with direct SQL query access
- Date presets and rolling windows
- Hooks system for extensibility
- `prx db query` for agent/power-user access
- `prx db raw` for inspecting raw API responses
- `prx export` for data portability
- `prx teams` for team management
- Proper binary distribution (no Node.js required)
