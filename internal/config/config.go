// Package config handles YAML configuration loading, validation, and defaults.
package config

// Config is the top-level configuration for prx.
type Config struct {
	Instances    map[string]Instance `yaml:"instances"`
	Teams        map[string]Team     `yaml:"teams,omitempty"`
	Repos        []RepoRef           `yaml:"repos,omitempty"`
	Fetch        FetchConfig         `yaml:"fetch,omitempty"`
	DateRange    DateRangeConfig     `yaml:"date_range,omitempty"`
	Output       OutputConfig        `yaml:"output,omitempty"`
	TestPatterns []string            `yaml:"test_patterns,omitempty"`
	Hooks        map[string][]Hook   `yaml:"hooks,omitempty"`
	Storage      StorageConfig       `yaml:"storage,omitempty"`
}

// Instance represents a GitHub (or future VCS) instance.
type Instance struct {
	Type          string      `yaml:"type"`
	BaseURL       string      `yaml:"base_url"`
	Token         TokenConfig `yaml:"token"`
	TLSSkipVerify bool        `yaml:"tls_skip_verify,omitempty"`
}

// TokenConfig defines how to resolve an authentication token.
type TokenConfig struct {
	Env string `yaml:"env"`
}

// Team maps a team name to a set of repos.
type Team struct {
	DisplayName string    `yaml:"display_name,omitempty"`
	Repos       []RepoRef `yaml:"repos"`
}

// RepoRef references a repository on a specific instance.
type RepoRef struct {
	Instance string `yaml:"instance"`
	Repo     string `yaml:"repo"`
}

// FetchConfig controls data fetching behavior.
type FetchConfig struct {
	States          []string `yaml:"states,omitempty"`
	PerPage         int      `yaml:"per_page,omitempty"`
	MaxRetries      int      `yaml:"max_retries,omitempty"`
	RateLimitBuffer int      `yaml:"rate_limit_buffer,omitempty"`
}

// DateRangeConfig defines the default date range for analysis.
type DateRangeConfig struct {
	Preset string `yaml:"preset,omitempty"`
	Start  string `yaml:"start,omitempty"`
	End    string `yaml:"end,omitempty"`
}

// OutputConfig controls report output.
type OutputConfig struct {
	Format    string `yaml:"format,omitempty"`
	Directory string `yaml:"directory,omitempty"`
	Timezone  string `yaml:"timezone,omitempty"`
}

// Hook defines a post-processing command.
type Hook struct {
	Name    string            `yaml:"name"`
	Command string            `yaml:"command"`
	Timeout int               `yaml:"timeout,omitempty"`
	Env     map[string]string `yaml:"env,omitempty"`
	OnError string            `yaml:"on_error,omitempty"`
}

// StorageConfig defines the storage backend.
type StorageConfig struct {
	Provider string       `yaml:"provider,omitempty"`
	SQLite   SQLiteConfig `yaml:"sqlite,omitempty"`
}

// SQLiteConfig holds SQLite-specific settings.
type SQLiteConfig struct {
	Path string `yaml:"path,omitempty"`
}
