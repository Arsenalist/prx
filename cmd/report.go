package cmd

import (
	"github.com/spf13/cobra"
)

var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "Fetch and analyze in one step (fetch + analyze)",
	RunE:  runReport,
}

func init() {
	reportCmd.Flags().StringSlice("team", nil, "team to report on")
	reportCmd.Flags().StringSlice("repo", nil, "repo(s) to report on")
	reportCmd.Flags().String("instance", "", "limit to repos from this instance")
	reportCmd.Flags().Bool("full", false, "re-fetch all data")
	reportCmd.Flags().String("start", "", "start date (YYYY-MM-DD)")
	reportCmd.Flags().String("end", "", "end date (YYYY-MM-DD)")
	reportCmd.Flags().String("preset", "", "date preset")
	reportCmd.Flags().String("group-by", "", "group results: repo, team, author")
	reportCmd.Flags().Bool("skip-fetch", false, "skip the fetch step")
	reportCmd.Flags().Bool("fetch-only", false, "stop after fetching")
	reportCmd.Flags().StringSlice("author", nil, "filter to specific author(s)")
	reportCmd.Flags().String("sort", "", "sort: prs-per-week, total-prs, avg-loc, avg-time-to-merge")
	reportCmd.Flags().Int("top", 0, "show only top N developers")
	reportCmd.Flags().String("output", "", "output directory for markdown files")
	rootCmd.AddCommand(reportCmd)
}

func runReport(cmd *cobra.Command, args []string) error {
	skipFetch, _ := cmd.Flags().GetBool("skip-fetch")
	fetchOnly, _ := cmd.Flags().GetBool("fetch-only")

	// Step 1: Fetch (unless --skip-fetch)
	if !skipFetch {
		if err := runFetch(cmd, args); err != nil {
			return err
		}
	}

	// Step 2: Analyze (unless --fetch-only)
	if fetchOnly {
		return nil
	}

	return runAnalyze(cmd, args)
}
