---
name: prx
description: >
  Use prx to fetch GitHub pull request data, store it locally in SQLite, and generate developer
  productivity metrics including velocity, timing, review quality, and collaboration stats.
  TRIGGER when: user asks about PR metrics, developer productivity, code review analytics,
  team performance, merge times, review turnaround, engineering reports, OR wants to create/manage
  teams ("create team X", "add repo to team"), add/remove repositories, fetch PR data for a
  time period ("fetch last 30 days", "get PRs since January"), generate reports or stats
  ("give me report for team X", "stats for repo Y", "what did alice work on"), compare developers
  or teams, export PR data, or check what data is available.
---

# prx — PR Analytics CLI

prx fetches pull request data from GitHub (including Enterprise), stores it in a local SQLite database, and computes developer productivity metrics, review analytics, and team reports.

## Agent Decision Workflow

Before running any command, follow this checklist:

1. **Does the database exist?** Run `prx status`. If it fails, run `prx init` first.
2. **Is an instance configured?** Run `prx instance list`. If empty, add one (see Setup).
3. **Are the repos tracked?** Run `prx repo list`. If the needed repo isn't listed, `prx repo add owner/repo --instance <name>`.
4. **Is data available for the requested time range?** Run `prx status` (or `prx status --team X`) to check last fetch time and PR counts. If data is stale or missing for the requested period, fetch it — but be smart (see Smart Fetching below).
5. **Run the actual command** (analyze, report, export, summarize).

## Natural Language → Commands

| User says | Commands to run |
|-----------|----------------|
| "create team X" | `prx team create X` |
| "add repo A to team X" | `prx team add-repo X owner/A` |
| "create team Y and add repos A, B, C" | `prx team create Y` then `prx team add-repo Y owner/A` (repeat for B, C) |
| "give me report for team X" | check status → fetch if needed → `prx report --team X --format json` |
| "give me stats for repo Y" | check status → fetch if needed → `prx analyze --repo owner/Y --format json` |
| "what did alice work on last month?" | check status → fetch if needed → `prx analyze --author alice --preset last-month --format json` |
| "compare teams A and B" | run `prx analyze --team A --format json` and `prx analyze --team B --format json` separately |
| "fetch PRs from last 90 days" | `prx fetch --preset last-90d` |
| "fetch PRs since March" | `prx fetch --since 2026-03-01` |

## Smart Fetching

Fetching is expensive — it makes GitHub API calls. Follow these rules:

- **Always check before fetching**: run `prx status --team X` or `prx status` to see when data was last fetched and how many PRs are stored. Only fetch if data is missing or stale for the requested period.
- **Use time bounds**: `prx fetch --since YYYY-MM-DD --until YYYY-MM-DD` to limit API calls to just the needed date range.
- **Use `--preset`**: `prx fetch --preset last-30d` for common ranges. Explicit `--since`/`--until` override preset values.
- **Scope by team or repo**: `prx fetch --team X` or `prx fetch --repo owner/repo` — don't fetch everything when you only need one team/repo.
- **Use `--dry-run` first if unsure**: shows what would be fetched without making API calls.
- **Incremental by default**: prx skips closed/merged PRs already in DB, so re-fetching is safe but still costs API calls for listing.
- **Always use `--format json`** when programmatically consuming analyze/report/export output.

## Binary Location

The `prx` binary is at the project root: `./prx`

If it doesn't exist, build it first:
```bash
go build -o prx .
```

## Database Resolution

prx resolves the database path in this order:
1. `--db <path>` flag
2. `$PRX_DB` environment variable
3. `~/.config/prx/prx.db` (default)

## Setup Workflow

Setting up prx requires these steps in order:

### 1. Initialize the database
```bash
prx init
```

### 2. Add a GitHub instance
```bash
# GitHub.com (most common)
prx instance add github --url https://api.github.com --token-env GITHUB_TOKEN

# GitHub Enterprise
prx instance add ghe --url https://ghe.company.com/api/v3 --token-env GHE_TOKEN --tls-skip-verify
```
The `--token-env` flag specifies the **environment variable name** (not the token itself) that holds the API token. You must set that environment variable before running `prx fetch`:

```bash
# macOS / Linux
export GITHUB_TOKEN=ghp_your_personal_access_token_here

# To persist across sessions, add to ~/.bashrc, ~/.zshrc, or ~/.profile:
echo 'export GITHUB_TOKEN=ghp_your_token' >> ~/.zshrc
```

```powershell
# Windows (PowerShell)
$env:GITHUB_TOKEN = "ghp_your_personal_access_token_here"

# To persist across sessions:
[System.Environment]::SetEnvironmentVariable("GITHUB_TOKEN", "ghp_your_token", "User")
```

```cmd
# Windows (Command Prompt)
set GITHUB_TOKEN=ghp_your_personal_access_token_here

# To persist across sessions:
setx GITHUB_TOKEN "ghp_your_token"
```

The token needs `repo` scope (for private repos) or just `public_repo` scope (for public repos only). Create one at GitHub > Settings > Developer settings > Personal access tokens.

### 3. Add repositories to track
```bash
prx repo add owner/repo --instance github
prx repo add org/another-repo --instance github
```

### 4. (Optional) Organize repos into teams
```bash
prx team create backend --display-name "Backend Team"
prx team add-repo backend owner/repo
prx team add-repo backend org/another-repo
```

## Core Commands

### Fetching Data
```bash
# Fetch all tracked repos (incremental — skips closed/merged PRs already stored)
prx fetch

# Fetch specific repo
prx fetch --repo owner/repo

# Fetch a team's repos
prx fetch --team backend

# Fetch with time bounds (filters by updated_at — catches PRs closed/merged in range)
prx fetch --preset last-30d
prx fetch --since 2026-03-01 --until 2026-04-01
prx fetch --team backend --preset last-90d
prx fetch --since 2026-01-01 --repo owner/repo

# Preset + explicit override (preset sets both bounds, explicit flags override either end)
prx fetch --preset last-90d --since 2026-01-15

# Full re-fetch (ignore incremental state)
prx fetch --full

# Dry run (show what would be fetched)
prx fetch --dry-run

# Verbose output (shows per-PR progress)
prx fetch --verbose
```

### Analyzing Metrics
```bash
# Analyze all data (defaults to last 30 days)
prx analyze

# Specific date range
prx analyze --start 2026-01-01 --end 2026-03-31

# Date presets: last-7d, last-14d, last-30d, last-90d, this-week, this-month, this-quarter
prx analyze --preset last-90d

# Filter by team, repo, or author
prx analyze --team backend
prx analyze --repo owner/repo
prx analyze --author alice --author bob

# Output formats: table (default), json, markdown, all
prx analyze --format json
prx analyze --format markdown --output ./reports/

# Sort developers: prs-per-week, total-prs, avg-loc, avg-time-to-merge
prx analyze --sort avg-time-to-merge --top 10
```

### One-Step Report (fetch + analyze)
```bash
prx report --team backend --preset last-30d --format markdown --output ./reports/
prx report --repo owner/repo --skip-fetch   # analyze only, don't fetch
prx report --repo owner/repo --fetch-only   # fetch only, don't analyze
```

### JSON Output for Agents
When using `--format json`, the output is structured as:
```json
{
  "meta": { "tool": "prx", "version": "0.1.0", "date_range": {...}, "repos": [...] },
  "summary": {
    "total_prs": 0, "merged_prs": 0, "closed_prs": 0, "open_prs": 0,
    "unique_authors": 0,
    "loc": { "total": 0, "test": 0, "production": 0, "avg_per_pr": 0, "median_per_pr": 0 },
    "timing": { "avg_time_to_open_hours": 0, "avg_time_to_merge_hours": 0, "avg_total_time_hours": 0, "median_time_to_merge_hours": 0 },
    "review": { "avg_time_to_first_review_hours": 0, "median_time_to_first_review_hours": 0, "avg_review_cycles": 0, "avg_comments_per_pr": 0 }
  },
  "developers": [...],
  "slowest_prs": [...],
  "reviewer_stats": [...],
  "prs": [
    {
      "repo": "owner/repo", "number": 1, "title": "PR title", "author": "alice",
      "state": "merged", "url": "https://...", "body": "PR description text",
      "created_at": "2026-01-01T00:00:00Z", "merged_at": "2026-01-02T00:00:00Z",
      "comment_count": 3, "review_comment_count": 2
    }
  ]
}
```

### Export Raw Data
```bash
prx export --format json --repo owner/repo --start 2026-01-01
prx export --export-format csv --output prs.csv --team backend
```

### Business Summary
```bash
# Generates a business-friendly JSON summary of merged PRs
prx summarize --team backend --preset last-30d
```

## Management Commands

### Instances
```bash
prx instance list
prx instance add <name> --url <base-url> --token-env <ENV_VAR>
prx instance remove <name>
```

### Repositories
```bash
prx repo list
prx repo add <owner/repo> --instance <name>
prx repo remove <owner/repo> --instance <name>
```

### Teams
```bash
prx team list
prx team create <name> --display-name "Display Name"
prx team show <name>
prx team add-repo <team> <owner/repo>
prx team remove-repo <team> <owner/repo>
prx team remove <name>
```

### Configuration
```bash
prx config list
prx config get <key>
prx config set <key> <value>
prx config reset <key>
```

Available config keys:
- `fetch.states` — JSON array, e.g. `'["closed","open"]'`
- `fetch.per_page` — integer (default: 100)
- `fetch.max_retries` — integer (default: 3)
- `date_range.preset` — last-7d, last-14d, last-30d, last-90d, this-week, etc.
- `date_range.start` / `date_range.end` — YYYY-MM-DD
- `output.format` — table, json, markdown, all
- `output.directory` — path for markdown reports
- `output.timezone` — e.g. America/Toronto
- `test_patterns` — JSON array of regex strings for classifying test files

## Database Access

For advanced queries against the SQLite database:
```bash
# Print database path
prx db path

# Show database stats
prx db stats

# Run arbitrary read-only SQL
prx db query "SELECT author, COUNT(*) as prs FROM pull_requests GROUP BY author ORDER BY prs DESC"

# Dump raw GitHub API JSON for a specific PR
prx db raw owner/repo 42
```

### Database Schema (key tables)
- `pull_requests` — id, repo_id, number, title, body, state, author, created_at, merged_at, additions, deletions, merged_by, comment_count, review_comment_count, raw_data
- `timeline_events` — id, pr_id, event_type, created_at, actor, review_state, raw_data
- `file_changes` — id, pr_id, filename, additions, deletions, is_test
- `branch_info` — pr_id, first_commit_date, commits_count, total_additions, total_deletions
- `repositories` — id, instance_id, owner, name, full_name
- `instances` — id, name, type, base_url, token_env
- `teams` — id, name, display_name

## Utility Commands

```bash
# Check what data is stored
prx status
prx status --team backend

# Backfill review fields from raw_data JSON (for data fetched before review metrics were added)
prx backfill

# Delete all data
prx reset

# Import legacy YAML config
prx import --config prx.yaml
```

## Common Patterns

**Quick team report:**
```bash
prx report --team backend --preset last-30d --format json
```

**Compare two developers:**
```bash
prx analyze --author alice --author bob --preset last-90d --format json
```

**Find slow PRs:**
```bash
prx analyze --sort avg-time-to-merge --format json | jq '.slowest_prs[:5]'
```

**Review health check:**
```bash
prx analyze --format json | jq '{review: .summary.review, reviewers: .reviewer_stats}'
```

**Export for external tools:**
```bash
prx export --export-format csv --output all-prs.csv --preset last-90d
```
