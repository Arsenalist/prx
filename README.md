# prx

PR analytics for engineering teams. Fetches pull request data from GitHub (including Enterprise), stores it locally in SQLite, and generates developer productivity metrics, team reports, and agent-consumable structured output.

## Installation

### From source (requires Go 1.21+)

```bash
go install github.com/Arsenalist/prx@latest
```

### Build from source

```bash
git clone https://github.com/Arsenalist/prx.git
cd prx
go build -o prx .
```

### Pre-built binaries

Download from the [Releases](https://github.com/Arsenalist/prx/releases) page. Binaries are available for Linux, macOS, and Windows (amd64 and arm64).

## Quick Start

### 1. Create a config file

```bash
prx init
```

This creates a `prx.yaml` in the current directory. Edit it to add your GitHub token and repos:

```yaml
instances:
  github:
    type: github
    base_url: https://api.github.com
    token:
      env: GITHUB_TOKEN

repos:
  - instance: github
    repo: your-org/your-repo
```

Set your token:

```bash
export GITHUB_TOKEN=ghp_your_token_here
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

prx uses a YAML config file. It looks for config in this order:

1. `--config` flag
2. `./prx.yaml` in the current directory
3. `$PRX_CONFIG` environment variable
4. `~/.config/prx/config.yaml`

Generate a starter config:

```bash
prx init              # full config with comments
prx init --minimal    # just the essentials
```

See [prx.example.yaml](prx.example.yaml) for a fully commented reference.

### GitHub instances

prx supports multiple GitHub instances (e.g., github.com + GitHub Enterprise):

```yaml
instances:
  github:
    type: github
    base_url: https://api.github.com
    token:
      env: GITHUB_TOKEN
  enterprise:
    type: github
    base_url: https://github.example.com/api/v3
    token:
      env: GHE_TOKEN
```

### Teams

Group repos into teams for consolidated reporting:

```yaml
teams:
  platform:
    display_name: Platform Team
    repos:
      - instance: github
        repo: org/api
      - instance: enterprise
        repo: corp/internal-service
```

### Date ranges

Set a default date range in config or override per-command:

```yaml
date_range:
  preset: last-30d
```

Available presets: `last-7d`, `last-14d`, `last-30d`, `last-90d`, `this-week`, `last-week`, `this-month`, `last-month`, `this-quarter`.

Or use explicit dates:

```yaml
date_range:
  start: "2026-01-01"
  end: "2026-03-31"
```

### Test file patterns

prx classifies LOC as test vs production using regex patterns:

```yaml
test_patterns:
  - "_test\\.go$"
  - "\\.test\\.(ts|js|tsx|jsx)$"
  - "\\.spec\\.(ts|js|tsx|jsx)$"
  - "/__tests__/"
```

### Hooks

Run commands after events. The analysis result JSON is piped to stdin:

```yaml
hooks:
  post-analyze:
    - name: slack-notify
      command: "curl -X POST $SLACK_WEBHOOK -d @-"
      timeout: 10
    - name: word-count
      command: "wc -c"
```

### Storage

prx stores data in SQLite by default:

```yaml
storage:
  provider: sqlite
  sqlite:
    path: ./prx.db
```

## Usage

### Getting help

Every command supports `--help`:

```bash
prx --help              # list all commands and global flags
prx fetch --help        # help for a specific command
prx db --help           # help for subcommand groups
```

### Global flags

| Flag | Description |
|------|-------------|
| `--config` | Path to config file |
| `--db` | Override database path |
| `--format` | Output format: `table` (default), `json`, `markdown`, `all` |
| `--quiet` | Suppress non-essential output |
| `--verbose` | Verbose/debug output |

### Fetching data

```bash
prx fetch                          # fetch all repos in config
prx fetch --repo org/repo          # fetch a specific repo
prx fetch --team platform          # fetch all repos in a team
prx fetch --full                   # re-fetch everything (ignore cache)
prx fetch --dry-run                # show what would be fetched
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

### Teams

```bash
prx teams                          # list all teams
prx teams show platform            # show repos in a team
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
