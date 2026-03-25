package config

import (
	"os"
	"path/filepath"
)

// DefaultTestPatterns are used when no test_patterns are configured.
var DefaultTestPatterns = []string{
	"/__tests__/",
	"/test/",
	"/tests/",
	"/spec/",
	"/specs/",
	"/src/test/",
	"\\.test\\.(ts|js|tsx|jsx)$",
	"\\.spec\\.(ts|js|tsx|jsx)$",
	"Test\\.java$",
	"Tests\\.java$",
	"TestCase\\.java$",
	"_test\\.go$",
}

// DefaultDBPath returns the default database path: ~/.config/prx/prx.db
func DefaultDBPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "prx.db"
	}
	return filepath.Join(home, ".config", "prx", "prx.db")
}

// ApplyDefaults fills in zero-value fields with sensible defaults.
func ApplyDefaults(cfg *Config) {
	if cfg.Fetch.PerPage == 0 {
		cfg.Fetch.PerPage = 100
	}
	if cfg.Fetch.MaxRetries == 0 {
		cfg.Fetch.MaxRetries = 3
	}
	if cfg.Fetch.RateLimitBuffer == 0 {
		cfg.Fetch.RateLimitBuffer = 100
	}
	if len(cfg.Fetch.States) == 0 {
		cfg.Fetch.States = []string{"closed", "open"}
	}
	if cfg.DateRange.Preset == "" && cfg.DateRange.Start == "" {
		cfg.DateRange.Preset = "last-30d"
	}
	if cfg.Output.Format == "" {
		cfg.Output.Format = "table"
	}
	if cfg.Output.Directory == "" {
		cfg.Output.Directory = "./reports"
	}
	if len(cfg.TestPatterns) == 0 {
		cfg.TestPatterns = DefaultTestPatterns
	}
	if cfg.Storage.Provider == "" {
		cfg.Storage.Provider = "sqlite"
	}
	if cfg.Storage.SQLite.Path == "" {
		cfg.Storage.SQLite.Path = DefaultDBPath()
	}

	for name, inst := range cfg.Instances {
		if inst.Type == "" {
			inst.Type = "github"
			cfg.Instances[name] = inst
		}
	}
}
