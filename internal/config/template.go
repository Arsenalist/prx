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
      env: GITHUB_TOKEN
    # tls_skip_verify: false

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
# Uncomment and adjust as needed:
#
# fetch:
#   states:
#     - closed
#     - open
#   per_page: 100
#   max_retries: 3
#   rate_limit_buffer: 100

# ─── Default Date Range ──────────────────────────────────────────
# Presets: last-7d, last-14d, last-30d, last-90d, this-week,
#          last-week, this-month, last-month, this-quarter
#
# date_range:
#   preset: "last-30d"
#   # OR use explicit dates:
#   # start: "2026-01-01"
#   # end: "2026-03-31"

# ─── Output Settings ─────────────────────────────────────────────
#
# output:
#   format: "table"
#   directory: "./reports"
#   timezone: "America/Toronto"

# ─── Test File Patterns ──────────────────────────────────────────
# Regex patterns to classify files as test vs production code.
# Defaults cover Go, JS/TS, and Java test conventions.
#
# test_patterns:
#   - "/__tests__/"
#   - "\\.test\\.(ts|js|tsx|jsx)$"
#   - "Test\\.java$"
#   - "_test\\.go$"

# ─── Storage ─────────────────────────────────────────────────────
#
# storage:
#   provider: "sqlite"
#   sqlite:
#     path: "~/.config/prx/prx.db"

# ─── Hooks ────────────────────────────────────────────────────────
# Shell commands run after fetch or analyze. Receive JSON on stdin.
#
# hooks:
#   post-analyze:
#     - name: "slack-notify"
#       command: "python3 scripts/slack.py"
#       timeout: 30
#     - name: "word-count"
#       command: "wc -c"
`
