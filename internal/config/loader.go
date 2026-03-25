package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var envVarPattern = regexp.MustCompile(`\$\{([^}]+)\}`)

// Load reads a config file, substitutes env vars, applies defaults, and validates.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	expanded, err := expandEnvVars(string(data))
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	ApplyDefaults(&cfg)

	if err := Validate(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// Resolve finds the config file path based on the resolution order:
// 1. Explicit path (from --config flag)
// 2. ./prx.yaml
// 3. $PRX_CONFIG env var
// 4. ~/.config/prx/config.yaml
//
// Returns (path, source, error) where source is one of "flag", "cwd", "env", "home".
func Resolve(explicit string) (string, string, error) {
	if explicit != "" {
		if _, err := os.Stat(explicit); err != nil {
			return "", "", fmt.Errorf("config file not found: %s", explicit)
		}
		return explicit, "flag", nil
	}

	if _, err := os.Stat("./prx.yaml"); err == nil {
		return "./prx.yaml", "cwd", nil
	}

	if envPath := os.Getenv("PRX_CONFIG"); envPath != "" {
		if _, err := os.Stat(envPath); err == nil {
			return envPath, "env", nil
		}
	}

	home, err := os.UserHomeDir()
	if err == nil {
		homePath := home + "/.config/prx/config.yaml"
		if _, err := os.Stat(homePath); err == nil {
			return homePath, "home", nil
		}
	}

	return "", "", fmt.Errorf("no config file found (tried ./prx.yaml, $PRX_CONFIG, ~/.config/prx/config.yaml)")
}

// expandEnvVars replaces ${VAR_NAME} references with environment variable values.
// Only expands in non-comment lines. Returns an error if any referenced variable is not set.
func expandEnvVars(input string) (string, error) {
	var missing []string
	var result []string

	for _, line := range strings.Split(input, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			// Leave comment lines untouched — don't expand env vars in comments
			result = append(result, line)
			continue
		}

		expanded := envVarPattern.ReplaceAllStringFunc(line, func(match string) string {
			varName := envVarPattern.FindStringSubmatch(match)[1]
			val, ok := os.LookupEnv(varName)
			if !ok {
				missing = append(missing, varName)
				return match
			}
			return val
		})
		result = append(result, expanded)
	}

	if len(missing) > 0 {
		return "", fmt.Errorf("undefined environment variable(s): %s", strings.Join(missing, ", "))
	}

	return strings.Join(result, "\n"), nil
}
