package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadMinimalConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "prx.yaml")
	yaml := `
instances:
  default:
    base_url: "https://api.github.com"
    token:
      env: GITHUB_TOKEN
repos:
  - instance: default
    repo: "owner/repo"
`
	require.NoError(t, os.WriteFile(path, []byte(yaml), 0644))

	cfg, err := Load(path)
	require.NoError(t, err)

	assert.Equal(t, "https://api.github.com", cfg.Instances["default"].BaseURL)
	assert.Equal(t, "github", cfg.Instances["default"].Type) // default applied
	assert.Equal(t, "GITHUB_TOKEN", cfg.Instances["default"].Token.Env)
	assert.Len(t, cfg.Repos, 1)
	assert.Equal(t, "owner/repo", cfg.Repos[0].Repo)
}

func TestLoadFullConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "prx.yaml")
	yaml := `
instances:
  github-com:
    type: github
    base_url: "https://api.github.com"
    token:
      env: GITHUB_TOKEN
  enterprise:
    type: github
    base_url: "https://github.mycompany.com/api/v3"
    token:
      env: GHE_TOKEN
    tls_skip_verify: true

teams:
  platform:
    display_name: "Platform Engineering"
    repos:
      - instance: enterprise
        repo: "org/platform-core"
      - instance: github-com
        repo: "company/oss-sdk"

repos:
  - instance: github-com
    repo: "owner/standalone"

fetch:
  states: ["all"]
  per_page: 50
  max_retries: 5
  rate_limit_buffer: 200

date_range:
  start: "2026-01-01"
  end: "2026-03-31"

output:
  format: json
  directory: "./output"
  timezone: "America/Toronto"

test_patterns:
  - "_test\\.go$"
  - "\\.test\\.ts$"

storage:
  provider: sqlite
  sqlite:
    path: "./local.db"

hooks:
  post-analyze:
    - name: "slack"
      command: "python3 slack.py"
      timeout: 60
      env:
        CHANNEL: "#eng"
      on_error: fail
`
	require.NoError(t, os.WriteFile(path, []byte(yaml), 0644))

	cfg, err := Load(path)
	require.NoError(t, err)

	// Instances
	assert.Len(t, cfg.Instances, 2)
	assert.True(t, cfg.Instances["enterprise"].TLSSkipVerify)

	// Teams
	assert.Len(t, cfg.Teams, 1)
	assert.Equal(t, "Platform Engineering", cfg.Teams["platform"].DisplayName)
	assert.Len(t, cfg.Teams["platform"].Repos, 2)

	// Fetch
	assert.Equal(t, []string{"all"}, cfg.Fetch.States)
	assert.Equal(t, 50, cfg.Fetch.PerPage)
	assert.Equal(t, 5, cfg.Fetch.MaxRetries)

	// Date range (no preset, explicit dates)
	assert.Equal(t, "2026-01-01", cfg.DateRange.Start)
	assert.Equal(t, "2026-03-31", cfg.DateRange.End)
	assert.Empty(t, cfg.DateRange.Preset) // no default preset when dates are set

	// Output
	assert.Equal(t, "json", cfg.Output.Format)

	// Test patterns (custom, not defaults)
	assert.Len(t, cfg.TestPatterns, 2)

	// Storage
	assert.Equal(t, "sqlite", cfg.Storage.Provider)
	assert.Equal(t, "./local.db", cfg.Storage.SQLite.Path)

	// Hooks
	hooks := cfg.Hooks["post-analyze"]
	require.Len(t, hooks, 1)
	assert.Equal(t, "slack", hooks[0].Name)
	assert.Equal(t, 60, hooks[0].Timeout)
	assert.Equal(t, "fail", hooks[0].OnError)
	assert.Equal(t, "#eng", hooks[0].Env["CHANNEL"])
}

func TestDefaultsApplied(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "prx.yaml")
	yaml := `
instances:
  default:
    base_url: "https://api.github.com"
    token:
      env: GITHUB_TOKEN
repos:
  - instance: default
    repo: "owner/repo"
`
	require.NoError(t, os.WriteFile(path, []byte(yaml), 0644))

	cfg, err := Load(path)
	require.NoError(t, err)

	assert.Equal(t, 100, cfg.Fetch.PerPage)
	assert.Equal(t, 3, cfg.Fetch.MaxRetries)
	assert.Equal(t, 100, cfg.Fetch.RateLimitBuffer)
	assert.Equal(t, []string{"closed", "open"}, cfg.Fetch.States)
	assert.Equal(t, "last-30d", cfg.DateRange.Preset)
	assert.Equal(t, "table", cfg.Output.Format)
	assert.Equal(t, "./reports", cfg.Output.Directory)
	assert.Equal(t, "sqlite", cfg.Storage.Provider)
	assert.NotEmpty(t, cfg.Storage.SQLite.Path)
	assert.Equal(t, DefaultTestPatterns, cfg.TestPatterns)
}

func TestEnvVarSubstitution(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "prx.yaml")
	yaml := `
instances:
  default:
    base_url: "${MY_GITHUB_URL}"
    token:
      env: GITHUB_TOKEN
repos:
  - instance: default
    repo: "owner/repo"
`
	require.NoError(t, os.WriteFile(path, []byte(yaml), 0644))
	t.Setenv("MY_GITHUB_URL", "https://custom.github.com/api/v3")

	cfg, err := Load(path)
	require.NoError(t, err)

	assert.Equal(t, "https://custom.github.com/api/v3", cfg.Instances["default"].BaseURL)
}

func TestEnvVarMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "prx.yaml")
	yaml := `
instances:
  default:
    base_url: "${NONEXISTENT_VAR_12345}"
    token:
      env: GITHUB_TOKEN
repos:
  - instance: default
    repo: "owner/repo"
`
	require.NoError(t, os.WriteFile(path, []byte(yaml), 0644))
	os.Unsetenv("NONEXISTENT_VAR_12345")

	_, err := Load(path)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "NONEXISTENT_VAR_12345")
}

func TestEnvVarInCommentIgnored(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "prx.yaml")
	yaml := `
instances:
  default:
    base_url: "https://api.github.com"
    token:
      env: GITHUB_TOKEN
repos:
  - instance: default
    repo: "owner/repo"
# hooks:
#   post-analyze:
#     - name: slack
#       command: "curl $SLACK_URL"
#       env:
#         WEBHOOK: "${TOTALLY_UNDEFINED_VAR}"
`
	require.NoError(t, os.WriteFile(path, []byte(yaml), 0644))
	os.Unsetenv("TOTALLY_UNDEFINED_VAR")

	cfg, err := Load(path)
	require.NoError(t, err)
	assert.NotNil(t, cfg)
}

func TestValidationNoInstances(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "prx.yaml")
	yaml := `
repos:
  - instance: default
    repo: "owner/repo"
`
	require.NoError(t, os.WriteFile(path, []byte(yaml), 0644))

	_, err := Load(path)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "at least one instance")
}

func TestValidationBadRepoFormat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "prx.yaml")
	yaml := `
instances:
  default:
    base_url: "https://api.github.com"
    token:
      env: GITHUB_TOKEN
repos:
  - instance: default
    repo: "just-a-name"
`
	require.NoError(t, os.WriteFile(path, []byte(yaml), 0644))

	_, err := Load(path)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "owner/repo")
}

func TestValidationOrphanInstanceRef(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "prx.yaml")
	yaml := `
instances:
  default:
    base_url: "https://api.github.com"
    token:
      env: GITHUB_TOKEN
repos:
  - instance: nonexistent
    repo: "owner/repo"
`
	require.NoError(t, os.WriteFile(path, []byte(yaml), 0644))

	_, err := Load(path)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent")
}

func TestValidationBadTestPattern(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "prx.yaml")
	yaml := `
instances:
  default:
    base_url: "https://api.github.com"
    token:
      env: GITHUB_TOKEN
repos:
  - instance: default
    repo: "owner/repo"
test_patterns:
  - "[invalid"
`
	require.NoError(t, os.WriteFile(path, []byte(yaml), 0644))

	_, err := Load(path)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "test pattern")
}

func TestValidationBadDateFormat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "prx.yaml")
	yaml := `
instances:
  default:
    base_url: "https://api.github.com"
    token:
      env: GITHUB_TOKEN
repos:
  - instance: default
    repo: "owner/repo"
date_range:
  start: "March 1"
`
	require.NoError(t, os.WriteFile(path, []byte(yaml), 0644))

	_, err := Load(path)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "YYYY-MM-DD")
}

func TestValidationInstanceMissingBaseURL(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "prx.yaml")
	yaml := `
instances:
  default:
    token:
      env: GITHUB_TOKEN
repos:
  - instance: default
    repo: "owner/repo"
`
	require.NoError(t, os.WriteFile(path, []byte(yaml), 0644))

	_, err := Load(path)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "base_url")
}

func TestValidationTeamOrphanInstance(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "prx.yaml")
	yaml := `
instances:
  default:
    base_url: "https://api.github.com"
    token:
      env: GITHUB_TOKEN
teams:
  myteam:
    repos:
      - instance: ghost
        repo: "org/repo"
`
	require.NoError(t, os.WriteFile(path, []byte(yaml), 0644))

	_, err := Load(path)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ghost")
}

func TestValidationBadOutputFormat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "prx.yaml")
	yaml := `
instances:
  default:
    base_url: "https://api.github.com"
    token:
      env: GITHUB_TOKEN
repos:
  - instance: default
    repo: "owner/repo"
output:
  format: "pdf"
`
	require.NoError(t, os.WriteFile(path, []byte(yaml), 0644))

	_, err := Load(path)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "format")
}

func TestConfigFileResolution(t *testing.T) {
	// When explicit path is given, it should use that
	_, err := Load("/nonexistent/path/prx.yaml")
	assert.Error(t, err)

	// Resolve should find a file in current dir or return error
	_, _, err = Resolve("")
	// This will either find ./prx.yaml or return an error — both are valid
	// We just test it doesn't panic
	_ = err
}

func TestResolveExplicitPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "custom.yaml")
	yaml := `
instances:
  default:
    base_url: "https://api.github.com"
    token:
      env: GITHUB_TOKEN
repos:
  - instance: default
    repo: "owner/repo"
`
	require.NoError(t, os.WriteFile(path, []byte(yaml), 0644))

	resolved, source, err := Resolve(path)
	require.NoError(t, err)
	assert.Equal(t, path, resolved)
	assert.Equal(t, "flag", source)
}
