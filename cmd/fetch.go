package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var fetchCmd = &cobra.Command{
	Use:   "fetch",
	Short: "Fetch PR data from GitHub and store locally",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("fetch: not implemented yet")
		return nil
	},
}

func init() {
	fetchCmd.Flags().StringSlice("team", nil, "fetch for a specific team's repos")
	fetchCmd.Flags().StringSlice("repo", nil, "fetch for specific repo(s) (owner/repo)")
	fetchCmd.Flags().String("instance", "", "limit to repos from this instance")
	fetchCmd.Flags().Bool("full", false, "re-fetch all data (ignore incremental state)")
	fetchCmd.Flags().Bool("dry-run", false, "show what would be fetched without making API calls")
	fetchCmd.Flags().String("since", "", "only fetch PRs updated after this date")
	fetchCmd.Flags().String("states", "", "PR states: open,closed,all")
	rootCmd.AddCommand(fetchCmd)
}
