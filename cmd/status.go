package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show overview of fetched data",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("status: not implemented yet")
		return nil
	},
}

func init() {
	statusCmd.Flags().StringSlice("team", nil, "show status for team's repos only")
	rootCmd.AddCommand(statusCmd)
}
