package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var teamsCmd = &cobra.Command{
	Use:   "teams",
	Short: "List and inspect team configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("teams: not implemented yet")
		return nil
	},
}

var teamsShowCmd = &cobra.Command{
	Use:   "show [name]",
	Short: "Show detailed team info",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("teams show %s: not implemented yet\n", args[0])
		return nil
	},
}

func init() {
	teamsCmd.AddCommand(teamsShowCmd)
	rootCmd.AddCommand(teamsCmd)
}
