package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export data from storage for debugging or portability",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("export: not implemented yet")
		return nil
	},
}

func init() {
	exportCmd.Flags().StringSlice("repo", nil, "export specific repo(s)")
	exportCmd.Flags().StringSlice("team", nil, "export team's repos")
	exportCmd.Flags().String("start", "", "filter by start date")
	exportCmd.Flags().String("end", "", "filter by end date")
	exportCmd.Flags().String("output", "", "output file (default: stdout)")
	rootCmd.AddCommand(exportCmd)
}
