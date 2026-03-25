package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var dbCmd = &cobra.Command{
	Use:   "db",
	Short: "Direct database access for power users and agents",
}

var dbQueryCmd = &cobra.Command{
	Use:   "query [sql]",
	Short: "Run a read-only SQL query against the database",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("db query: not implemented yet (sql: %s)\n", args[0])
		return nil
	},
}

var dbPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Print the database file path",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("db path: not implemented yet")
		return nil
	},
}

var dbStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show database size and table row counts",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("db stats: not implemented yet")
		return nil
	},
}

var dbRawCmd = &cobra.Command{
	Use:   "raw [repo] [pr-number]",
	Short: "Dump raw JSON blob for a specific PR",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("db raw: not implemented yet (repo: %s, pr: %s)\n", args[0], args[1])
		return nil
	},
}

func init() {
	dbCmd.AddCommand(dbQueryCmd)
	dbCmd.AddCommand(dbPathCmd)
	dbCmd.AddCommand(dbStatsCmd)
	dbCmd.AddCommand(dbRawCmd)
	rootCmd.AddCommand(dbCmd)
}
