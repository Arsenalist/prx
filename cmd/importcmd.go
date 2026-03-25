package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Arsenalist/prx/internal/config"
	"github.com/Arsenalist/prx/internal/store"
	"github.com/spf13/cobra"
)

var importCmd = &cobra.Command{
	Use:   "import [path-to-prx.yaml]",
	Short: "Import configuration from a YAML file into the database",
	Long:  `Reads an existing prx.yaml config file and writes instances, repos, teams, and settings into the database.`,
	Args:  cobra.MaximumNArgs(1),
	RunE:  runImport,
}

func init() {
	rootCmd.AddCommand(importCmd)
}

func runImport(cmd *cobra.Command, args []string) error {
	// Resolve YAML path
	var yamlPath string
	if len(args) > 0 {
		yamlPath = args[0]
	} else {
		path, _, err := config.Resolve(cfgFile)
		if err != nil {
			return fmt.Errorf("no YAML file specified and none found: %w", err)
		}
		yamlPath = path
	}

	cfg, err := config.Load(yamlPath)
	if err != nil {
		return fmt.Errorf("loading YAML: %w", err)
	}

	db, cleanup, err := openDB()
	if err != nil {
		return err
	}
	defer cleanup()

	// Import instances
	instIDs := make(map[string]int64)
	for name, inst := range cfg.Instances {
		id, err := db.UpsertInstance(store.InstanceRecord{
			Name:          name,
			Type:          inst.Type,
			BaseURL:       inst.BaseURL,
			TokenEnv:      inst.Token.Env,
			TLSSkipVerify: inst.TLSSkipVerify,
		})
		if err != nil {
			return fmt.Errorf("importing instance %q: %w", name, err)
		}
		instIDs[name] = id
		fmt.Printf("  Instance: %s\n", name)
	}

	// Import repos (standalone + team repos)
	repoIDs := make(map[string]int64) // fullName -> ID
	allRefs := cfg.Repos
	for _, team := range cfg.Teams {
		allRefs = append(allRefs, team.Repos...)
	}

	for _, ref := range allRefs {
		if _, done := repoIDs[ref.Repo]; done {
			continue
		}
		instID, ok := instIDs[ref.Instance]
		if !ok {
			fmt.Printf("  Warning: skipping repo %s (instance %q not found)\n", ref.Repo, ref.Instance)
			continue
		}
		parts := strings.SplitN(ref.Repo, "/", 2)
		if len(parts) != 2 {
			continue
		}
		id, err := db.UpsertRepository(store.RepositoryRecord{
			InstanceID: instID,
			Owner:      parts[0],
			Name:       parts[1],
			FullName:   ref.Repo,
		})
		if err != nil {
			return fmt.Errorf("importing repo %s: %w", ref.Repo, err)
		}
		repoIDs[ref.Repo] = id
		fmt.Printf("  Repo: %s\n", ref.Repo)
	}

	// Import teams
	for name, team := range cfg.Teams {
		teamID, err := db.UpsertTeam(store.TeamRecord{
			Name:        name,
			DisplayName: team.DisplayName,
		})
		if err != nil {
			return fmt.Errorf("importing team %q: %w", name, err)
		}
		for _, ref := range team.Repos {
			if repoID, ok := repoIDs[ref.Repo]; ok {
				db.AddTeamRepo(teamID, repoID)
			}
		}
		fmt.Printf("  Team: %s (%d repos)\n", name, len(team.Repos))
	}

	// Import settings
	settingsCount := 0

	if len(cfg.Fetch.States) > 0 {
		data, _ := json.Marshal(cfg.Fetch.States)
		db.SetSetting("fetch.states", string(data))
		settingsCount++
	}
	if cfg.Fetch.PerPage != 0 && cfg.Fetch.PerPage != 100 {
		db.SetSetting("fetch.per_page", fmt.Sprintf("%d", cfg.Fetch.PerPage))
		settingsCount++
	}
	if cfg.Fetch.MaxRetries != 0 && cfg.Fetch.MaxRetries != 3 {
		db.SetSetting("fetch.max_retries", fmt.Sprintf("%d", cfg.Fetch.MaxRetries))
		settingsCount++
	}
	if cfg.Fetch.RateLimitBuffer != 0 && cfg.Fetch.RateLimitBuffer != 100 {
		db.SetSetting("fetch.rate_limit_buffer", fmt.Sprintf("%d", cfg.Fetch.RateLimitBuffer))
		settingsCount++
	}

	if cfg.DateRange.Preset != "" && cfg.DateRange.Preset != "last-30d" {
		db.SetSetting("date_range.preset", cfg.DateRange.Preset)
		settingsCount++
	}
	if cfg.DateRange.Start != "" {
		db.SetSetting("date_range.start", cfg.DateRange.Start)
		settingsCount++
	}
	if cfg.DateRange.End != "" {
		db.SetSetting("date_range.end", cfg.DateRange.End)
		settingsCount++
	}

	if cfg.Output.Format != "" && cfg.Output.Format != "table" {
		db.SetSetting("output.format", cfg.Output.Format)
		settingsCount++
	}
	if cfg.Output.Directory != "" && cfg.Output.Directory != "./reports" {
		db.SetSetting("output.directory", cfg.Output.Directory)
		settingsCount++
	}
	if cfg.Output.Timezone != "" {
		db.SetSetting("output.timezone", cfg.Output.Timezone)
		settingsCount++
	}

	if len(cfg.TestPatterns) > 0 {
		data, _ := json.Marshal(cfg.TestPatterns)
		defaultData, _ := json.Marshal(config.DefaultTestPatterns)
		if string(data) != string(defaultData) {
			db.SetSetting("test_patterns", string(data))
			settingsCount++
		}
	}

	if len(cfg.Hooks) > 0 {
		data, _ := json.Marshal(cfg.Hooks)
		db.SetSetting("hooks", string(data))
		settingsCount++
	}

	if settingsCount > 0 {
		fmt.Printf("  Settings: %d imported\n", settingsCount)
	}

	fmt.Printf("\nImport complete from %s\n", yamlPath)
	return nil
}
