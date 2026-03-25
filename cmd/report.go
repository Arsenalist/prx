package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "Fetch and analyze in one step (fetch + analyze)",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("report: not implemented yet")
		return nil
	},
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
	rootCmd.AddCommand(reportCmd)
}
