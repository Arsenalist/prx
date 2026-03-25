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

### 1. Initialize and configure

```bash
prx init
```

Then add a GitHub instance and some repos:

```bash
export GITHUB_TOKEN=ghp_your_token_here

prx instance add github --url https://api.github.com --token-env GITHUB_TOKEN
prx repo add your-org/your-repo --instance github
```

### 2. Fetch PR data

```bash
prx fetch
```

### 3. View metrics

```bash
prx analyze
```

That's it. You'll see a table with summary stats, developer metrics, and the slowest PRs.

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

### GitHub instances

prx supports multiple GitHub instances (e.g., github.com + GitHub Enterprise):

```bash
# Add GitHub.com
prx instance add github --url https://api.github.com --token-env GITHUB_TOKEN

# Add GitHub Enterprise
prx instance add enterprise --url https://github.example.com/api/v3 --token-env GHE_TOKEN --tls-skip-verify

# List instances
prx instance list

# Remove an instance
prx instance remove enterprise
```

### Repositories

```bash
# Add a repo to track
prx repo add org/api --instance github
prx repo add org/web --instance github

# List tracked repos
prx repo list

# Remove a repo
prx repo remove org/web
```

### Teams

Group repos into teams for consolidated reporting:

```bash
# Create a team
prx team create platform --display-name "Platform Engineering"

# Add repos to the team
prx team add-repo platform org/api
prx team add-repo platform org/web

# List teams
prx team list

# Show team details
prx team show platform

# Remove a repo from a team
prx team remove-repo platform org/web

# Delete a team
prx team remove platform
```

### Settings

Global settings are stored in the database. Use `prx config` to manage them:

```bash
# View all settings
prx config list

# Set a value
prx config set date_range.preset last-30d
prx config set fetch.states '["closed","open"]'
prx config set output.format json
prx config set output.directory ./reports

# Get a specific setting
prx config get fetch.per_page

# Reset to default
prx config reset output.format
```

Available settings:

| Key | Default | Description |
|-----|---------|-------------|
| `fetch.states` | `["closed"]` | PR states to fetch |
| `fetch.per_page` | `100` | PRs per API page |
| `fetch.max_retries` | `3` | API retry count |
| `fetch.rate_limit_buffer` | `100` | Warn when remaining requests drop below this |
| `date_range.preset` | `last-30d` | Default date preset |
| `date_range.start` | | Explicit start date (YYYY-MM-DD) |
| `date_range.end` | | Explicit end date (YYYY-MM-DD) |
| `output.format` | `table` | Default output format |
| `output.directory` | `./reports` | Directory for markdown reports |
| `output.timezone` | | Timezone for reports |
| `test_patterns` | (Go/JS/Java defaults) | JSON array of regex patterns |
| `hooks` | | JSON object of hook definitions |

### Date range presets

Available presets: `last-7d`, `last-14d`, `last-30d`, `last-90d`, `this-week`, `last-week`, `this-month`, `last-month`, `this-quarter`.

### Test file patterns

prx classifies LOC as test vs production using regex patterns. The defaults cover Go, JavaScript/TypeScript, and Java conventions. Customize with:

```bash
prx config set test_patterns '["_test\\.go$", "\\.test\\.(ts|js)$", "/__tests__/"]'
```

### Hooks

Run commands after events. The analysis result JSON is piped to stdin:

```bash
prx config set hooks '{"post-analyze": [{"name": "slack", "command": "curl -X POST $SLACK_WEBHOOK -d @-", "timeout": 10}]}'
```

## Usage

### Getting help

Every command supports `--help`:

```bash
prx --help              # list all commands and global flags
prx fetch --help        # help for a specific command
prx team --help         # help for subcommand groups
```

### Global flags

| Flag | Description |
|------|-------------|
| `--db` | Override database path |
| `--format` | Output format: `table` (default), `json`, `markdown`, `all` |
| `--quiet` | Suppress non-essential output |
| `--verbose` | Verbose/debug output |

### Fetching data

```bash
prx fetch                          # fetch all repos
prx fetch --repo org/repo          # fetch a specific repo
prx fetch --team platform          # fetch all repos in a team
prx fetch --full                   # re-fetch everything (ignore cache)
prx fetch --dry-run                # show what would be fetched
prx fetch --verbose                # detailed per-PR progress
```

prx uses smart sync: closed/merged PRs already in the database are skipped, only open PRs are re-fetched.

### Analyzing metrics

```bash
prx analyze                                  # analyze all data
prx analyze --preset last-30d                # last 30 days
prx analyze --start 2026-01-01 --end 2026-03-31
prx analyze --team platform                  # filter by team
prx analyze --author alice --author bob      # filter by author
prx analyze --format json                    # JSON output
prx analyze --format markdown --output ./reports
```

### Combined fetch + analyze

```bash
prx report                         # fetch then analyze
prx report --team platform --preset last-month
prx report --skip-fetch            # analyze only (same as prx analyze)
prx report --fetch-only            # fetch only (same as prx fetch)
```

### JSON summary for agents

```bash
prx summarize --preset last-30d    # JSON to stdout, pipe-friendly
prx summarize | jq '.summary.total_prs'
```

### Exporting data

```bash
prx export                                    # JSON to stdout
prx export --export-format csv --output prs.csv
prx export --team platform --preset last-90d
```

### Direct database access

```bash
prx db path                        # print database file path
prx db stats                       # show table row counts
prx db query "SELECT count(*) as n FROM pull_requests"
prx db raw org/repo 123            # dump raw JSON blob for PR #123
```

### Version

```bash
prx version
```

## Output formats

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

## License

MIT
