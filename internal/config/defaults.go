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

