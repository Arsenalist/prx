package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var analyzeCmd = &cobra.Command{
	Use:   "analyze",
	Short: "Calculate and display metrics from fetched PR data",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("analyze: not implemented yet")
		return nil
	},
}

func init() {
	analyzeCmd.Flags().StringSlice("team", nil, "analyze a specific team (repeatable)")
	analyzeCmd.Flags().StringSlice("repo", nil, "analyze specific repo(s) (repeatable)")
	analyzeCmd.Flags().StringSlice("author", nil, "filter to specific author(s)")
	analyzeCmd.Flags().String("start", "", "start date (YYYY-MM-DD)")
	analyzeCmd.Flags().String("end", "", "end date (YYYY-MM-DD)")
	analyzeCmd.Flags().String("preset", "", "date preset (e.g., last-30d, this-week)")
	analyzeCmd.Flags().String("group-by", "", "group results: repo, team, author")
	analyzeCmd.Flags().String("sort", "", "sort: prs-per-week, total-prs, avg-loc, avg-time-to-merge")
	analyzeCmd.Flags().Int("top", 0, "show only top N developers")
	analyzeCmd.Flags().String("output", "./reports", "output directory for markdown files")
	rootCmd.AddCommand(analyzeCmd)
}
