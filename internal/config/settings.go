package config

import (
	"encoding/json"
	"strconv"
)

// SettingsReader is the subset of store.Store needed to load settings.
type SettingsReader interface {
	GetSetting(key string) (string, error)
	ListSettings() (map[string]string, error)
}

// Settings holds runtime configuration loaded from the database.
type Settings struct {
	Fetch        FetchConfig
	DateRange    DateRangeConfig
	Output       OutputConfig
	TestPatterns []string
	Hooks        map[string][]Hook
}

// LoadSettings reads configuration from the database settings table and applies defaults.
func LoadSettings(db SettingsReader) (*Settings, error) {
	all, err := db.ListSettings()
	if err != nil {
		return nil, err
	}

	s := &Settings{
		Hooks: make(map[string][]Hook),
	}

	// Fetch
	s.Fetch.States = getStringSlice(all, "fetch.states", []string{"closed"})
	s.Fetch.PerPage = getInt(all, "fetch.per_page", 100)
	s.Fetch.MaxRetries = getInt(all, "fetch.max_retries", 3)
	s.Fetch.RateLimitBuffer = getInt(all, "fetch.rate_limit_buffer", 100)

	// Date range
	s.DateRange.Preset = getString(all, "date_range.preset", "last-30d")
	s.DateRange.Start = getString(all, "date_range.start", "")
	s.DateRange.End = getString(all, "date_range.end", "")
	// If explicit dates are set, clear preset
	if s.DateRange.Start != "" || s.DateRange.End != "" {
		s.DateRange.Preset = ""
	}

	// Output
	s.Output.Format = getString(all, "output.format", "table")
	s.Output.Directory = getString(all, "output.directory", "./reports")
	s.Output.Timezone = getString(all, "output.timezone", "")

	// Test patterns
	s.TestPatterns = getStringSlice(all, "test_patterns", DefaultTestPatterns)

	// Hooks
	if raw, ok := all["hooks"]; ok && raw != "" {
		_ = json.Unmarshal([]byte(raw), &s.Hooks)
	}

	return s, nil
}

func getString(m map[string]string, key, defaultVal string) string {
	if v, ok := m[key]; ok && v != "" {
		return v
	}
	return defaultVal
}

func getInt(m map[string]string, key string, defaultVal int) int {
	if v, ok := m[key]; ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return defaultVal
}

func getStringSlice(m map[string]string, key string, defaultVal []string) []string {
	if v, ok := m[key]; ok && v != "" {
		var result []string
		if err := json.Unmarshal([]byte(v), &result); err == nil && len(result) > 0 {
			return result
		}
	}
	return defaultVal
}
