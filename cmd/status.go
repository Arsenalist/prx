package cmd

import (
	"fmt"

	"github.com/Arsenalist/prx/internal/store/sqlite"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show overview of fetched data",
	RunE:  runStatus,
}

func init() {
	statusCmd.Flags().StringSlice("team", nil, "show status for team's repos only")
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	db := sqlite.New(cfg.Storage.SQLite.Path)
	if err := db.Open(); err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer db.Close()
	if err := db.Migrate(); err != nil {
		return fmt.Errorf("migrating database: %w", err)
	}

	stats, err := db.Stats()
	if err != nil {
		return fmt.Errorf("getting stats: %w", err)
	}

	fmt.Printf("Database: %s", stats.DatabasePath)
	if stats.SizeBytes > 0 {
		fmt.Printf(" (%.1f KB)", float64(stats.SizeBytes)/1024)
	}
	fmt.Println()
	fmt.Println()

	repos, err := db.ListRepositories()
	if err != nil {
		return err
	}

	if len(repos) == 0 {
		fmt.Println("No data fetched yet. Run `prx fetch` to get started.")
		return nil
	}

	fmt.Printf("  %-40s  %5s  %-20s\n", "Repository", "PRs", "Last Fetched")
	fmt.Printf("  %-40s  %5s  %-20s\n", "────────────────────────────────────────", "─────", "────────────────────")

	totalPRs := 0
	for _, repo := range repos {
		meta, err := db.GetFetchMetadata(repo.ID)
		if err != nil {
			continue
		}

		prCount := 0
		lastFetch := "never"
		if meta != nil {
			prCount = meta.PRCount
			lastFetch = meta.LastFetchAt
		}
		totalPRs += prCount

		fmt.Printf("  %-40s  %5d  %-20s\n", repo.FullName, prCount, lastFetch)
	}

	fmt.Println()
	fmt.Printf("Total: %d repos, %d PRs\n", len(repos), totalPRs)

	return nil
}
