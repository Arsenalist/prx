package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/Arsenalist/prx/internal/config"
	"github.com/spf13/cobra"
)

var teamsCmd = &cobra.Command{
	Use:   "teams",
	Short: "List and inspect team configuration",
	RunE:  runTeamsList,
}

var teamsShowCmd = &cobra.Command{
	Use:   "show [name]",
	Short: "Show detailed team info",
	Args:  cobra.ExactArgs(1),
	RunE:  runTeamsShow,
}

func init() {
	teamsCmd.AddCommand(teamsShowCmd)
	rootCmd.AddCommand(teamsCmd)
}

func runTeamsList(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	if len(cfg.Teams) == 0 {
		fmt.Fprintln(os.Stderr, "No teams defined in config.")
		return nil
	}

	for name, team := range cfg.Teams {
		instances := countInstances(team)
		display := name
		if team.DisplayName != "" {
			display = fmt.Sprintf("%s (%s)", team.DisplayName, name)
		}
		fmt.Printf("  %s — %d repos across %d instance(s)\n", display, len(team.Repos), instances)
	}
	return nil
}

func runTeamsShow(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	name := args[0]
	team, ok := cfg.Teams[name]
	if !ok {
		return fmt.Errorf("team %q not found in config", name)
	}

	display := name
	if team.DisplayName != "" {
		display = team.DisplayName
	}
	fmt.Printf("Team: %s\n", display)
	fmt.Printf("Repos (%d):\n", len(team.Repos))
	for _, r := range team.Repos {
		fmt.Printf("  %s (instance: %s)\n", r.Repo, r.Instance)
	}
	return nil
}

// resolveTeamRepos expands --team flags to repo references using config.
func resolveTeamRepos(cfg *config.Config, teamFlags []string) []repoRef {
	var refs []repoRef
	for _, teamName := range teamFlags {
		team, ok := cfg.Teams[teamName]
		if !ok {
			fmt.Fprintf(os.Stderr, "Warning: team %q not found in config, skipping\n", teamName)
			continue
		}
		for _, r := range team.Repos {
			refs = append(refs, repoRef{instance: r.Instance, repo: r.Repo})
		}
	}
	return refs
}

func countInstances(team config.Team) int {
	seen := make(map[string]bool)
	for _, r := range team.Repos {
		seen[r.Instance] = true
	}
	return len(seen)
}

// teamName returns the team name for metadata (if a single team is selected).
func teamNameFromFlags(teamFlags []string) string {
	if len(teamFlags) == 1 {
		return teamFlags[0]
	}
	if len(teamFlags) > 1 {
		return strings.Join(teamFlags, "+")
	}
	return ""
}
