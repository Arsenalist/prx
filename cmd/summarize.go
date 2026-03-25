package cmd

import (
	"fmt"

	"github.com/Arsenalist/prx/internal/metrics"
	"github.com/Arsenalist/prx/internal/report"
	"github.com/Arsenalist/prx/internal/store"
	"github.com/Arsenalist/prx/internal/store/sqlite"
	"github.com/spf13/cobra"
)

var summarizeCmd = &cobra.Command{
	Use:   "summarize",
	Short: "Generate business-friendly JSON summary of merged PRs",
	RunE:  runSummarize,
}

func init() {
	summarizeCmd.Flags().StringSlice("team", nil, "summarize for a team")
	summarizeCmd.Flags().StringSlice("repo", nil, "summarize specific repo(s)")
	summarizeCmd.Flags().StringSlice("author", nil, "filter to specific author(s)")
	summarizeCmd.Flags().String("start", "", "start date (YYYY-MM-DD)")
	summarizeCmd.Flags().String("end", "", "end date (YYYY-MM-DD)")
	summarizeCmd.Flags().String("preset", "", "date preset")
	rootCmd.AddCommand(summarizeCmd)
}

func runSummarize(cmd *cobra.Command, args []string) error {
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

	startDate, endDate := resolveDateRange(cmd, cfg)
	repoFlags, _ := cmd.Flags().GetStringSlice("repo")
	authors, _ := cmd.Flags().GetStringSlice("author")

	startStr := startDate.Format("2006-01-02")
	endStr := endDate.Format("2006-01-02")

	filters := store.PRFilters{
		Authors:   authors,
		StartDate: startStr,
		EndDate:   endStr,
	}

	repoNames, err := resolveRepoIDs(db, cfg, repoFlags, &filters)
	if err != nil {
		return err
	}

	prs, err := db.ListPullRequests(filters)
	if err != nil {
		return fmt.Errorf("listing PRs: %w", err)
	}

	if len(prs) == 0 {
		fmt.Println("{}")
		return nil
	}

	result := metrics.Calculate(prs, db, metrics.CalculateOptions{
		StartDate: startStr,
		EndDate:   endStr,
		Repos:     repoNames,
	})

	// Summarize always outputs JSON (it's the agent-friendly command)
	output, err := report.FormatJSON(result)
	if err != nil {
		return err
	}
	fmt.Println(output)
	return nil
}
