package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var summarizeCmd = &cobra.Command{
	Use:   "summarize",
	Short: "Generate business-friendly summaries of merged PRs",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("summarize: not implemented yet")
		return nil
	},
}

func init() {
	summarizeCmd.Flags().StringSlice("team", nil, "summarize for a team")
	summarizeCmd.Flags().StringSlice("repo", nil, "summarize specific repo(s)")
	summarizeCmd.Flags().String("start", "", "start date (YYYY-MM-DD)")
	summarizeCmd.Flags().String("end", "", "end date (YYYY-MM-DD)")
	summarizeCmd.Flags().String("preset", "", "date preset")
	rootCmd.AddCommand(summarizeCmd)
}
