package config

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockSettingsReader is a fake DB for testing LoadSettings.
type mockSettingsReader struct {
	settings map[string]string
}

func (m *mockSettingsReader) GetSetting(key string) (string, error) {
	return m.settings[key], nil
}

func (m *mockSettingsReader) ListSettings() (map[string]string, error) {
	return m.settings, nil
}

func TestLoadSettingsDefaults(t *testing.T) {
	db := &mockSettingsReader{settings: map[string]string{}}

	s, err := LoadSettings(db)
	require.NoError(t, err)

	// Fetch defaults
	assert.Equal(t, []string{"closed"}, s.Fetch.States)
	assert.Equal(t, 100, s.Fetch.PerPage)
	assert.Equal(t, 3, s.Fetch.MaxRetries)
	assert.Equal(t, 100, s.Fetch.RateLimitBuffer)

	// Date range defaults
	assert.Equal(t, "last-30d", s.DateRange.Preset)
	assert.Empty(t, s.DateRange.Start)
	assert.Empty(t, s.DateRange.End)

	// Output defaults
	assert.Equal(t, "table", s.Output.Format)
	assert.Equal(t, "./reports", s.Output.Directory)
	assert.Empty(t, s.Output.Timezone)

	// Test patterns defaults
	assert.Equal(t, DefaultTestPatterns, s.TestPatterns)

	// Hooks defaults
	assert.Empty(t, s.Hooks)
}

func TestLoadSettingsCustomValues(t *testing.T) {
	states, _ := json.Marshal([]string{"open", "closed"})
	patterns, _ := json.Marshal([]string{"_test\\.go$"})
	hooks, _ := json.Marshal(map[string][]Hook{
		"post-analyze": {{Name: "slack", Command: "notify.sh", Timeout: 30}},
	})

	db := &mockSettingsReader{settings: map[string]string{
		"fetch.states":           string(states),
		"fetch.per_page":         "50",
		"fetch.max_retries":      "5",
		"fetch.rate_limit_buffer": "200",
		"date_range.preset":      "last-7d",
		"output.format":          "json",
		"output.directory":       "/tmp/reports",
		"output.timezone":        "America/Toronto",
		"test_patterns":          string(patterns),
		"hooks":                  string(hooks),
	}}

	s, err := LoadSettings(db)
	require.NoError(t, err)

	assert.Equal(t, []string{"open", "closed"}, s.Fetch.States)
	assert.Equal(t, 50, s.Fetch.PerPage)
	assert.Equal(t, 5, s.Fetch.MaxRetries)
	assert.Equal(t, 200, s.Fetch.RateLimitBuffer)
	assert.Equal(t, "last-7d", s.DateRange.Preset)
	assert.Equal(t, "json", s.Output.Format)
	assert.Equal(t, "/tmp/reports", s.Output.Directory)
	assert.Equal(t, "America/Toronto", s.Output.Timezone)
	assert.Equal(t, []string{"_test\\.go$"}, s.TestPatterns)
	require.Len(t, s.Hooks["post-analyze"], 1)
	assert.Equal(t, "slack", s.Hooks["post-analyze"][0].Name)
}

func TestLoadSettingsExplicitDatesClearPreset(t *testing.T) {
	db := &mockSettingsReader{settings: map[string]string{
		"date_range.preset": "last-90d",
		"date_range.start":  "2026-01-01",
	}}

	s, err := LoadSettings(db)
	require.NoError(t, err)

	// Preset should be cleared when explicit dates are set
	assert.Empty(t, s.DateRange.Preset)
	assert.Equal(t, "2026-01-01", s.DateRange.Start)
}

func TestLoadSettingsInvalidIntFallsBack(t *testing.T) {
	db := &mockSettingsReader{settings: map[string]string{
		"fetch.per_page": "not-a-number",
	}}

	s, err := LoadSettings(db)
	require.NoError(t, err)

	assert.Equal(t, 100, s.Fetch.PerPage) // default
}
