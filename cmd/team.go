package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/Arsenalist/prx/internal/store"
	"github.com/Arsenalist/prx/internal/store/sqlite"
	"github.com/spf13/cobra"
)

var teamCmd = &cobra.Command{
	Use:   "team",
	Short: "Manage teams",
}

var teamCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a team",
	Args:  cobra.ExactArgs(1),
	RunE:  runTeamCreate,
}

var teamRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove a team",
	Args:  cobra.ExactArgs(1),
	RunE:  runTeamRemove,
}

var teamAddRepoCmd = &cobra.Command{
	Use:   "add-repo <team> <owner/repo>",
	Short: "Add a repository to a team",
	Args:  cobra.ExactArgs(2),
	RunE:  runTeamAddRepo,
}

var teamRemoveRepoCmd = &cobra.Command{
	Use:   "remove-repo <team> <owner/repo>",
	Short: "Remove a repository from a team",
	Args:  cobra.ExactArgs(2),
	RunE:  runTeamRemoveRepo,
}

var teamListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all teams",
	RunE:  runTeamList,
}

var teamShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Show team details",
	Args:  cobra.ExactArgs(1),
	RunE:  runTeamShow,
}

func init() {
	teamCreateCmd.Flags().String("display-name", "", "team display name")

	teamCmd.AddCommand(teamCreateCmd)
	teamCmd.AddCommand(teamRemoveCmd)
	teamCmd.AddCommand(teamAddRepoCmd)
	teamCmd.AddCommand(teamRemoveRepoCmd)
	teamCmd.AddCommand(teamListCmd)
	teamCmd.AddCommand(teamShowCmd)
	rootCmd.AddCommand(teamCmd)
}

func runTeamCreate(cmd *cobra.Command, args []string) error {
	db, cleanup, err := openDB()
	if err != nil {
		return err
	}
	defer cleanup()

	name := args[0]
	displayName, _ := cmd.Flags().GetString("display-name")

	_, err = db.UpsertTeam(store.TeamRecord{
		Name:        name,
		DisplayName: displayName,
	})
	if err != nil {
		return fmt.Errorf("creating team: %w", err)
	}

	fmt.Printf("Team %q created.\n", name)
	return nil
}

func runTeamRemove(cmd *cobra.Command, args []string) error {
	db, cleanup, err := openDB()
	if err != nil {
		return err
	}
	defer cleanup()

	name := args[0]
	if err := db.DeleteTeam(name); err != nil {
		return fmt.Errorf("removing team: %w", err)
	}

	fmt.Printf("Team %q removed.\n", name)
	return nil
}

func runTeamAddRepo(cmd *cobra.Command, args []string) error {
	db, cleanup, err := openDB()
	if err != nil {
		return err
	}
	defer cleanup()

	teamName := args[0]
	repoFullName := args[1]

	parts := strings.SplitN(repoFullName, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return fmt.Errorf("repo must be in owner/repo format, got %q", repoFullName)
	}

	team, err := db.GetTeamByName(teamName)
	if err != nil {
		return err
	}
	if team == nil {
		return fmt.Errorf("team %q not found", teamName)
	}

	// Find repo by full name
	repos, err := db.FindRepositoriesByFullName(repoFullName)
	if err != nil {
		return err
	}
	if len(repos) == 0 {
		return fmt.Errorf("repository %q not found (use `prx repo add` first)", repoFullName)
	}
	if len(repos) > 1 {
		return fmt.Errorf("repository %q exists in multiple instances; use `prx team add-repo` with a specific instance", repoFullName)
	}

	if err := db.AddTeamRepo(team.ID, repos[0].ID); err != nil {
		return fmt.Errorf("adding repo to team: %w", err)
	}

	fmt.Printf("Added %q to team %q.\n", repoFullName, teamName)
	return nil
}

func runTeamRemoveRepo(cmd *cobra.Command, args []string) error {
	db, cleanup, err := openDB()
	if err != nil {
		return err
	}
	defer cleanup()

	teamName := args[0]
	repoFullName := args[1]

	team, err := db.GetTeamByName(teamName)
	if err != nil {
		return err
	}
	if team == nil {
		return fmt.Errorf("team %q not found", teamName)
	}

	repos, err := db.FindRepositoriesByFullName(repoFullName)
	if err != nil {
		return err
	}
	if len(repos) == 0 {
		return fmt.Errorf("repository %q not found", repoFullName)
	}
	if len(repos) > 1 {
		return fmt.Errorf("repository %q exists in multiple instances; specify which to remove", repoFullName)
	}

	if err := db.RemoveTeamRepo(team.ID, repos[0].ID); err != nil {
		return fmt.Errorf("removing repo from team: %w", err)
	}

	fmt.Printf("Removed %q from team %q.\n", repoFullName, teamName)
	return nil
}

func runTeamList(cmd *cobra.Command, args []string) error {
	db, cleanup, err := openDB()
	if err != nil {
		return err
	}
	defer cleanup()

	teams, err := db.ListTeams()
	if err != nil {
		return err
	}

	if len(teams) == 0 {
		fmt.Println("No teams defined. Use `prx team create` to add one.")
		return nil
	}

	for _, team := range teams {
		repos, _ := db.GetTeamRepos(team.ID)
		display := team.Name
		if team.DisplayName != "" {
			display = fmt.Sprintf("%s (%s)", team.DisplayName, team.Name)
		}
		fmt.Printf("  %s — %d repos\n", display, len(repos))
	}
	return nil
}

func runTeamShow(cmd *cobra.Command, args []string) error {
	db, cleanup, err := openDB()
	if err != nil {
		return err
	}
	defer cleanup()

	name := args[0]
	team, err := db.GetTeamByName(name)
	if err != nil {
		return err
	}
	if team == nil {
		return fmt.Errorf("team %q not found", name)
	}

	display := team.Name
	if team.DisplayName != "" {
		display = team.DisplayName
	}
	fmt.Printf("Team: %s\n", display)

	repos, err := db.GetTeamRepos(team.ID)
	if err != nil {
		return err
	}

	fmt.Printf("Repos (%d):\n", len(repos))
	for _, r := range repos {
		fmt.Printf("  %s\n", r.FullName)
	}
	return nil
}

// resolveTeamReposFromDB expands team names to repository records using the DB.
// Returns full RepositoryRecord values so callers can use IDs directly.
func resolveTeamReposFromDB(db *sqlite.SQLiteStore, teamFlags []string) ([]store.RepositoryRecord, error) {
	var repos []store.RepositoryRecord
	for _, teamName := range teamFlags {
		team, err := db.GetTeamByName(teamName)
		if err != nil {
			return nil, err
		}
		if team == nil {
			fmt.Fprintf(os.Stderr, "Warning: team %q not found, skipping\n", teamName)
			continue
		}
		teamRepos, err := db.GetTeamRepos(team.ID)
		if err != nil {
			return nil, err
		}
		repos = append(repos, teamRepos...)
	}
	return repos, nil
}

// teamNameFromFlags returns the team name for metadata (if a single team is selected).
func teamNameFromFlags(teamFlags []string) string {
	if len(teamFlags) == 1 {
		return teamFlags[0]
	}
	if len(teamFlags) > 1 {
		return strings.Join(teamFlags, "+")
	}
	return ""
}
