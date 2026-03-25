package cmd

import (
	"fmt"
	"strings"

	"github.com/Arsenalist/prx/internal/store"
	"github.com/spf13/cobra"
)

var repoCmd = &cobra.Command{
	Use:   "repo",
	Short: "Manage repositories",
}

var repoAddCmd = &cobra.Command{
	Use:   "add <owner/repo>",
	Short: "Add a repository to track",
	Args:  cobra.ExactArgs(1),
	RunE:  runRepoAdd,
}

var repoRemoveCmd = &cobra.Command{
	Use:   "remove <owner/repo>",
	Short: "Remove a repository",
	Args:  cobra.ExactArgs(1),
	RunE:  runRepoRemove,
}

var repoListCmd = &cobra.Command{
	Use:   "list",
	Short: "List tracked repositories",
	RunE:  runRepoList,
}

func init() {
	repoAddCmd.Flags().String("instance", "", "instance name (required)")
	repoAddCmd.MarkFlagRequired("instance")

	repoCmd.AddCommand(repoAddCmd)
	repoCmd.AddCommand(repoRemoveCmd)
	repoCmd.AddCommand(repoListCmd)
	rootCmd.AddCommand(repoCmd)
}

func runRepoAdd(cmd *cobra.Command, args []string) error {
	db, cleanup, err := openDB()
	if err != nil {
		return err
	}
	defer cleanup()

	fullName := args[0]
	parts := strings.SplitN(fullName, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return fmt.Errorf("repo must be in owner/repo format, got %q", fullName)
	}

	instanceName, _ := cmd.Flags().GetString("instance")
	inst, err := db.GetInstanceByName(instanceName)
	if err != nil {
		return err
	}
	if inst == nil {
		return fmt.Errorf("instance %q not found (use `prx instance add` first)", instanceName)
	}

	_, err = db.UpsertRepository(store.RepositoryRecord{
		InstanceID: inst.ID,
		Owner:      parts[0],
		Name:       parts[1],
		FullName:   fullName,
	})
	if err != nil {
		return fmt.Errorf("adding repository: %w", err)
	}

	fmt.Printf("Repository %q added to instance %q.\n", fullName, instanceName)
	return nil
}

func runRepoRemove(cmd *cobra.Command, args []string) error {
	db, cleanup, err := openDB()
	if err != nil {
		return err
	}
	defer cleanup()

	fullName := args[0]

	// Find the repo to get its instance ID
	repos, err := db.ListRepositories()
	if err != nil {
		return err
	}

	found := false
	for _, r := range repos {
		if r.FullName == fullName {
			if err := db.DeleteRepository(r.InstanceID, fullName); err != nil {
				return fmt.Errorf("removing repository: %w", err)
			}
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("repository %q not found", fullName)
	}

	fmt.Printf("Repository %q removed.\n", fullName)
	return nil
}

func runRepoList(cmd *cobra.Command, args []string) error {
	db, cleanup, err := openDB()
	if err != nil {
		return err
	}
	defer cleanup()

	repos, err := db.ListRepositories()
	if err != nil {
		return err
	}

	if len(repos) == 0 {
		fmt.Println("No repositories configured. Use `prx repo add` to add one.")
		return nil
	}

	// Build instance name map
	instances, _ := db.ListInstances()
	instMap := make(map[int64]string)
	for _, inst := range instances {
		instMap[inst.ID] = inst.Name
	}

	for _, r := range repos {
		instName := instMap[r.InstanceID]
		fmt.Printf("  %-40s (instance: %s)\n", r.FullName, instName)
	}
	return nil
}
