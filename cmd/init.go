package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Generate a starter prx.yaml config file",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("init: not implemented yet")
		return nil
	},
}

func init() {
	initCmd.Flags().String("path", "./prx.yaml", "where to create config")
	initCmd.Flags().Bool("minimal", false, "generate minimal config")
	rootCmd.AddCommand(initCmd)
}
