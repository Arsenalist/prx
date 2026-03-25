package config

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	repoPattern = regexp.MustCompile(`^[^/]+/[^/]+$`)
	datePattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	validFormats = map[string]bool{
		"table": true, "json": true, "markdown": true, "all": true,
	}
)

// Validate checks a loaded config for correctness.
func Validate(cfg *Config) error {
	if len(cfg.Instances) == 0 {
		return fmt.Errorf("config: at least one instance must be defined")
	}

	for name, inst := range cfg.Instances {
		if inst.BaseURL == "" {
			return fmt.Errorf("config: instance %q is missing base_url", name)
		}
	}

	// Validate standalone repos
	for _, ref := range cfg.Repos {
		if err := validateRepoRef(cfg, ref, "repos"); err != nil {
			return err
		}
	}

	// Validate team repos
	for teamName, team := range cfg.Teams {
		for _, ref := range team.Repos {
			if err := validateRepoRef(cfg, ref, fmt.Sprintf("team %q", teamName)); err != nil {
				return err
			}
		}
	}

	// Validate date range
	if cfg.DateRange.Start != "" && !datePattern.MatchString(cfg.DateRange.Start) {
		return fmt.Errorf("config: date_range.start must be YYYY-MM-DD, got %q", cfg.DateRange.Start)
	}
	if cfg.DateRange.End != "" && !datePattern.MatchString(cfg.DateRange.End) {
		return fmt.Errorf("config: date_range.end must be YYYY-MM-DD, got %q", cfg.DateRange.End)
	}

	// Validate output format
	if !validFormats[cfg.Output.Format] {
		return fmt.Errorf("config: output format must be one of: %s, got %q",
			strings.Join(formatKeys(), ", "), cfg.Output.Format)
	}

	// Validate test patterns compile as regex
	for _, pattern := range cfg.TestPatterns {
		if _, err := regexp.Compile(pattern); err != nil {
			return fmt.Errorf("config: invalid test pattern %q: %w", pattern, err)
		}
	}

	return nil
}

func validateRepoRef(cfg *Config, ref RepoRef, context string) error {
	if _, ok := cfg.Instances[ref.Instance]; !ok {
		return fmt.Errorf("config: %s references undefined instance %q", context, ref.Instance)
	}
	if !repoPattern.MatchString(ref.Repo) {
		return fmt.Errorf("config: %s repo %q must be in owner/repo format", context, ref.Repo)
	}
	return nil
}

func formatKeys() []string {
	keys := make([]string, 0, len(validFormats))
	for k := range validFormats {
		keys = append(keys, k)
	}
	return keys
}
