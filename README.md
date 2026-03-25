# prx

PR analytics for engineering teams. Fetches pull request data from GitHub (including Enterprise), stores it locally in SQLite, and generates developer productivity metrics, team reports, and agent-consumable structured output.

## Installation

### From source (requires Go 1.21+)

```bash
go install github.com/Arsenalist/prx@latest
```

Make sure `$GOPATH/bin` is in your `PATH`. If `GOPATH` is not set, Go defaults to `~/go`:

```bash
export PATH="$PATH:$(go env GOPATH)/bin"
```

Add that line to your `~/.zshrc` or `~/.bashrc` to make it permanent.

### Build from source

```bash
git clone https://github.com/Arsenalist/prx.git
cd prx
go build -o prx .
```

### Pre-built binaries

Download from the [Releases](https://github.com/Arsenalist/prx/releases) page. Binaries are available for Linux, macOS, and Windows (amd64 and arm64).

## Quick Start

```bash
# 1. Initialize the database
prx init

# 2. Add a GitHub instance
export GITHUB_TOKEN=ghp_your_token_here
prx instance add github --url https://api.github.com --token-env GITHUB_TOKEN

# 3. Add repos to track
prx repo add myorg/api --instance github
prx repo add myorg/web --instance github

# 4. Fetch PR data
prx fetch

# 5. View metrics
prx analyze
```

## Configuration

All configuration is stored in a local SQLite database. No YAML files needed — manage everything via CLI commands.

The database location is resolved in this order:
1. `--db` flag
2. `$PRX_DB` environment variable
3. `~/.config/prx/prx.db` (default)

### Migrating from YAML

If you have an existing `prx.yaml` config file, import it:

```bash
prx import prx.yaml
```

This migrates instances, repos, teams, and settings into the database.

---

## Managing Instances

An instance is a GitHub API endpoint. You need at least one.

```bash
# Add GitHub.com
prx instance add github --url https://api.github.com --token-env GITHUB_TOKEN

# Add GitHub Enterprise
prx instance add enterprise \
  --url https://github.example.com/api/v3 \
  --token-env GHE_TOKEN \
  --tls-skip-verify

# Update an existing instance (upserts by name)
prx instance add github --url https://api.github.com --token-env NEW_TOKEN_VAR

# List all instances
prx instance list
#   github               https://api.github.com (token: $GITHUB_TOKEN)
#   enterprise           https://github.example.com/api/v3 (token: $GHE_TOKEN) [tls-skip-verify]

# Remove an instance
prx instance remove enterprise
```

The `--token-env` flag specifies the **name** of the environment variable holding your token (not the token itself). Set it in your shell:

```bash
export GITHUB_TOKEN=ghp_abc123
export GHE_TOKEN=ghp_def456
```

---

## Managing Repositories

Repositories are tied to an instance. You must add an instance before adding repos.

```bash
# Add repos to the "github" instance
prx repo add myorg/api --instance github
prx repo add myorg/web --instance github
prx repo add myorg/mobile --instance github

# Add a repo from GitHub Enterprise
prx repo add corp/internal-service --instance enterprise

# List all tracked repos
prx repo list
#   myorg/api                                (instance: github)
#   myorg/web                                (instance: github)
#   myorg/mobile                             (instance: github)
#   corp/internal-service                    (instance: enterprise)

# Remove a repo (stops tracking, keeps existing data in DB)
prx repo remove myorg/mobile
```

---

## Managing Teams

Teams group repos together for consolidated fetching, analysis, and reporting. A repo can belong to multiple teams.

### Creating teams and adding repos

```bash
# Create a team
prx team create platform --display-name "Platform Engineering"
prx team create frontend --display-name "Frontend Team"

# Add repos to the platform team
prx team add-repo platform myorg/api
prx team add-repo platform myorg/web

# Add repos to the frontend team (repos can be in multiple teams)
prx team add-repo frontend myorg/web
prx team add-repo frontend myorg/mobile

# Adding the same repo twice is a no-op (idempotent)
prx team add-repo platform myorg/api
```

### Viewing teams

```bash
# List all teams with repo counts
prx team list
#   Platform Engineering (platform) — 2 repos
#   Frontend Team (frontend) — 2 repos

# Show which repos are in a team
prx team show platform
#   Team: Platform Engineering
#   Repos (2):
#     myorg/api
#     myorg/web

prx team show frontend
#   Team: Frontend Team
#   Repos (2):
#     myorg/web
#     myorg/mobile
```

### Removing repos from teams and deleting teams

```bash
# Remove a single repo from a team (does not delete the repo itself)
prx team remove-repo frontend myorg/mobile

# Verify it's gone
prx team show frontend
#   Team: Frontend Team
#   Repos (1):
#     myorg/web

# Delete an entire team (does not delete the repos, just the grouping)
prx team remove frontend
```

---

## Fetching Data

### Fetch everything

```bash
# Fetch all tracked repos
prx fetch
```

### Fetch a single repo

```bash
prx fetch --repo myorg/api
```

### Fetch multiple specific repos

```bash
prx fetch --repo myorg/api --repo myorg/web
```

### Fetch by team

```bash
# Fetch all repos in the platform team
prx fetch --team platform

# Fetch repos from multiple teams
prx fetch --team platform --team frontend
```

### Fetch modes

```bash
# Incremental (default) — skips closed/merged PRs already in DB
prx fetch

# Full re-fetch — re-downloads everything
prx fetch --full

# Dry run — shows what would be fetched without making API calls
prx fetch --dry-run

# Verbose — shows per-PR progress and API call details
prx fetch --verbose
```

prx uses smart sync: closed/merged PRs already in the database are skipped. Only open PRs are re-fetched to check for updates.

---

## Analyzing Metrics

### Analyze everything

```bash
# All repos, default date range (last 30 days)
prx analyze
```

### Analyze by team

```bash
# Only repos in the platform team
prx analyze --team platform

# Multiple teams combined
prx analyze --team platform --team frontend
```

### Analyze a single repo

```bash
prx analyze --repo myorg/api
```

### Analyze multiple specific repos

```bash
prx analyze --repo myorg/api --repo myorg/web
```

### Filter by author

```bash
# Single author
prx analyze --author alice

# Multiple authors
prx analyze --author alice --author bob

# Combine with team filter
prx analyze --team platform --author alice
```

### Date ranges

```bash
# Presets
prx analyze --preset last-7d
prx analyze --preset last-30d
prx analyze --preset this-month
prx analyze --preset last-quarter

# Explicit dates
prx analyze --start 2026-01-01 --end 2026-03-31

# CLI flags override the default preset in settings
```

Available presets: `last-7d`, `last-14d`, `last-30d`, `last-90d`, `this-week`, `last-week`, `this-month`, `last-month`, `this-quarter`.

### Output formats

```bash
# Table (default) — human-readable
prx analyze

# JSON — structured, pipe-friendly
prx analyze --format json
prx analyze --format json | jq '.developers[] | {name, prs_per_week}'

# Markdown — written to a file
prx analyze --format markdown --output ./reports

# All — table to stderr, JSON to stdout, markdown to file
prx analyze --format all --output ./reports
```

---

## Combined Fetch + Analyze

The `report` command runs fetch followed by analyze in a single step:

```bash
# Fetch and analyze all repos
prx report

# Fetch and analyze a specific team
prx report --team platform --preset last-month

# Fetch and analyze specific repos
prx report --repo myorg/api --repo myorg/web --preset last-7d

# Skip fetch (analyze only, same as prx analyze)
prx report --skip-fetch

# Fetch only (same as prx fetch)
prx report --fetch-only
```

---

## JSON Summary for Agents

`summarize` always outputs JSON, designed for piping to AI agents or scripts:

```bash
prx summarize --preset last-30d
prx summarize --team platform
prx summarize --repo myorg/api --author alice
prx summarize | jq '.summary.total_prs'
```

---

## Exporting Raw Data

Export raw PR records as JSON or CSV:

```bash
# JSON to stdout (all repos, default date range)
prx export

# CSV to a file
prx export --export-format csv --output prs.csv

# Export a specific team's data
prx export --team platform --preset last-90d

# Export specific repos
prx export --repo myorg/api --repo myorg/web

# Export with date range
prx export --start 2026-01-01 --end 2026-03-31 --export-format csv --output q1.csv
```

---

## Settings

Global settings are stored in the database. They control defaults for fetching, date ranges, output, and more.

```bash
# View all custom settings
prx config list

# Set a value
prx config set date_range.preset last-30d
prx config set output.format json

# Get a specific setting (shows default if not set)
prx config get fetch.per_page

# Reset to default
prx config reset output.format
```

### Available settings

| Key | Default | Description |
|-----|---------|-------------|
| `fetch.states` | `["closed"]` | PR states to fetch. Use `'["closed","open"]'` for both. |
| `fetch.per_page` | `100` | PRs per API page |
| `fetch.max_retries` | `3` | API retry count |
| `fetch.rate_limit_buffer` | `100` | Warn when remaining API requests drop below this |
| `date_range.preset` | `last-30d` | Default date preset for analysis |
| `date_range.start` | | Explicit start date (YYYY-MM-DD) |
| `date_range.end` | | Explicit end date (YYYY-MM-DD) |
| `output.format` | `table` | Default output format: `table`, `json`, `markdown`, `all` |
| `output.directory` | `./reports` | Directory for markdown report files |
| `output.timezone` | | Timezone for reports (e.g., `America/Toronto`) |
| `test_patterns` | (Go/JS/Java defaults) | JSON array of regex patterns to classify test files |
| `hooks` | | JSON object of hook definitions (see Hooks section) |

### Examples

```bash
# Fetch both open and closed PRs
prx config set fetch.states '["closed","open"]'

# Set default analysis window to 90 days
prx config set date_range.preset last-90d

# Always output JSON
prx config set output.format json

# Custom test patterns for a Python project
prx config set test_patterns '["test_.*\\.py$", "/tests/", "_test\\.py$"]'
```

### Test file patterns

prx classifies each file change as test or production code using regex patterns. The defaults cover Go (`_test.go`), JavaScript/TypeScript (`.test.ts`, `.spec.js`, `__tests__/`), and Java (`Test.java`). Override them for your stack:

```bash
# Python project
prx config set test_patterns '["test_.*\\.py$", "/tests/", "_test\\.py$"]'

# Ruby project
prx config set test_patterns '["_spec\\.rb$", "/spec/", "_test\\.rb$"]'

# Reset to built-in defaults
prx config reset test_patterns
```

### Hooks

Run shell commands after events. The analysis result JSON is piped to the command's stdin:

```bash
prx config set hooks '{"post-analyze": [{"name": "slack", "command": "curl -X POST $SLACK_WEBHOOK -d @-", "timeout": 10}]}'
```

---

## Direct Database Access

Power users and AI agents can query the SQLite database directly:

```bash
# Print the database file path
prx db path

# Show table sizes
prx db stats

# Run arbitrary SQL queries (read-only)
prx db query "SELECT author, count(*) as prs FROM pull_requests GROUP BY author ORDER BY prs DESC LIMIT 10"
prx db query "SELECT count(*) as n FROM pull_requests WHERE state = 'merged'"

# Dump the raw GitHub API JSON blob for a specific PR
prx db raw myorg/api 123
```

---

## Typical Workflows

### Single repo, quick check

```bash
prx instance add github --url https://api.github.com --token-env GITHUB_TOKEN
prx repo add myorg/api --instance github
prx report --preset last-7d
```

### Multi-repo team setup

```bash
# Setup
prx instance add github --url https://api.github.com --token-env GITHUB_TOKEN
prx repo add myorg/api --instance github
prx repo add myorg/web --instance github
prx repo add myorg/mobile --instance github
prx team create platform --display-name "Platform Engineering"
prx team add-repo platform myorg/api
prx team add-repo platform myorg/web

# Weekly team report
prx report --team platform --preset last-week

# Monthly report as markdown
prx report --team platform --preset last-month --format markdown --output ./reports

# Compare two repos
prx analyze --repo myorg/api --repo myorg/web --preset last-30d
```

### Multi-instance (GitHub.com + Enterprise)

```bash
prx instance add github --url https://api.github.com --token-env GITHUB_TOKEN
prx instance add enterprise --url https://github.corp.com/api/v3 --token-env GHE_TOKEN
prx repo add oss/sdk --instance github
prx repo add corp/backend --instance enterprise
prx team create all-repos
prx team add-repo all-repos oss/sdk
prx team add-repo all-repos corp/backend
prx report --team all-repos
```

### Exporting for external analysis

```bash
# CSV for spreadsheets
prx export --team platform --preset last-90d --export-format csv --output platform-q1.csv

# JSON for scripts
prx export --repo myorg/api --start 2026-01-01 --end 2026-03-31 | jq '.[].author' | sort | uniq -c | sort -rn
```

---

## Global Flags

| Flag | Description |
|------|-------------|
| `--db` | Override database path |
| `--format` | Output format: `table` (default), `json`, `markdown`, `all` |
| `--quiet` | Suppress non-essential output |
| `--verbose` | Verbose/debug output |

## Output Formats

| Format | Description |
|--------|-------------|
| `table` | Human-readable ASCII table (default) |
| `json` | Structured JSON, pipe-friendly for agents and `jq` |
| `markdown` | Markdown report written to file |
| `all` | Table to stderr, JSON to stdout, markdown to file |

## Metrics

prx calculates the following metrics:

- **Summary**: total PRs, merged/closed/open counts, unique authors, total LOC (test vs production), average and median LOC per PR, average and median time to merge
- **Developer stats**: merged PRs, PRs per week, average LOC (total/test/production), average time to open, time to merge, and total time
- **Slowest PRs**: ranked by total time, showing time-to-open, draft time, time-to-merge, and LOC

## Getting Help

Every command and subcommand supports `--help`:

```bash
prx --help
prx instance --help
prx instance add --help
prx team --help
prx team add-repo --help
prx config set --help
prx fetch --help
prx analyze --help
```

## License

MIT
