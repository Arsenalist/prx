// Package config handles configuration loading and defaults.
package config

// FetchConfig controls data fetching behavior.
type FetchConfig struct {
	States          []string
	PerPage         int
	MaxRetries      int
	RateLimitBuffer int
}

// DateRangeConfig defines the default date range for analysis.
type DateRangeConfig struct {
	Preset string
	Start  string
	End    string
}

// OutputConfig controls report output.
type OutputConfig struct {
	Format    string
	Directory string
	Timezone  string
}

// Hook defines a post-processing command.
type Hook struct {
	Name    string
	Command string
	Timeout int
	Env     map[string]string
	OnError string
}
