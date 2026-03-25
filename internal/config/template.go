package config

// MinimalTemplate is the starter config for `prx init --minimal`.
const MinimalTemplate = `instances:
  default:
    type: github
    base_url: "https://api.github.com"
    token:
      env: GITHUB_TOKEN

repos:
  - instance: default
    repo: "owner/repo"
`

// FullTemplate is the commented starter config for `prx init`.
const FullTemplate = `# prx configuration
# Docs: https://github.com/Arsenalist/prx

# ─── GitHub Instances ─────────────────────────────────────────────
# Define one or more GitHub instances (supports GitHub.com and Enterprise).
instances:
  default:
    type: github
    base_url: "https://api.github.com"
    token:
      env: GITHUB_TOKEN                   # Environment variable holding your token
    # tls_skip_verify: false              # Set true for self-signed certs

  # enterprise:
  #   type: github
  #   base_url: "https://github.mycompany.com/api/v3"
  #   token:
  #     env: GHE_TOKEN

# ─── Teams ────────────────────────────────────────────────────────
# Map team names to repositories for team-based reporting.
# teams:
#   platform:
#     display_name: "Platform Engineering"
#     repos:
#       - instance: default
#         repo: "org/platform-core"
#       - instance: default
#         repo: "org/platform-infra"

# ─── Standalone Repos ─────────────────────────────────────────────
# Repos not assigned to any team.
repos:
  - instance: default
    repo: "owner/repo"

# ─── Fetch Settings ──────────────────────────────────────────────
# fetch:
#   states: ["closed", "open"]            # PR states to fetch
#   per_page: 100                         # Results per API page (max 100)
#   max_retries: 3                        # Retry count on transient failures
#   rate_limit_buffer: 100                # Warn when remaining requests below this

# ─── Default Date Range ──────────────────────────────────────────
# date_range:
#   preset: "last-30d"                    # Options: today, this-week, last-week,
#                                         #   this-month, last-month, this-quarter,
#                                         #   last-quarter, this-year, last-7d,
#                                         #   last-14d, last-30d, last-60d, last-90d
#   # OR use explicit dates:
#   # start: "2026-01-01"
#   # end: "2026-03-31"

# ─── Output Settings ─────────────────────────────────────────────
# output:
#   format: "table"                       # table, json, markdown, all
#   directory: "./reports"                # Directory for markdown files
#   timezone: "America/Toronto"

# ─── Test File Patterns ──────────────────────────────────────────
# Regex patterns to classify files as test vs production code.
# Defaults cover JS/TS, Java, and Go test conventions.
# test_patterns:
#   - "/__tests__/"
#   - "\\.test\\.(ts|js|tsx|jsx)$"
#   - "Test\\.java$"
#   - "_test\\.go$"

# ─── Storage ─────────────────────────────────────────────────────
# storage:
#   provider: "sqlite"
#   sqlite:
#     path: "~/.config/prx/prx.db"

# ─── Hooks ────────────────────────────────────────────────────────
# Shell commands run after fetch or analyze. Receive JSON on stdin.
# hooks:
#   post-analyze:
#     - name: "slack-notify"
#       command: "python3 scripts/slack.py"
#       timeout: 30
#       env:
#         SLACK_WEBHOOK: "${SLACK_WEBHOOK_URL}"
#       on_error: warn                    # warn (default) or fail
`
