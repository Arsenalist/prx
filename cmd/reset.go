package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var resetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Delete all data from the database",
	Long:  `Deletes all instances, repositories, teams, pull requests, and settings from the database. The database file and schema are preserved.`,
	RunE:  runReset,
}

func init() {
	resetCmd.Flags().Bool("force", false, "skip confirmation prompt")
	rootCmd.AddCommand(resetCmd)
}

func runReset(cmd *cobra.Command, args []string) error {
	force, _ := cmd.Flags().GetBool("force")

	db, cleanup, err := openDB()
	if err != nil {
		return err
	}
	defer cleanup()

	// Show what will be deleted
	stats, err := db.Stats()
	if err != nil {
		return err
	}

	totalRows := int64(0)
	for _, count := range stats.Tables {
		totalRows += count
	}

	if totalRows == 0 {
		fmt.Println("Database is already empty.")
		return nil
	}

	fmt.Fprintf(os.Stderr, "This will delete ALL data from %s:\n\n", stats.DatabasePath)
	for table, count := range stats.Tables {
		if count > 0 {
			fmt.Fprintf(os.Stderr, "  %-20s %d rows\n", table, count)
		}
	}
	fmt.Fprintln(os.Stderr)

	if !force {
		fmt.Fprint(os.Stderr, "Are you sure? Type 'yes' to confirm: ")
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(answer)
		if answer != "yes" {
			fmt.Fprintln(os.Stderr, "Aborted.")
			return nil
		}
	}

	if err := db.ResetAll(); err != nil {
		return fmt.Errorf("resetting database: %w", err)
	}

	fmt.Fprintln(os.Stderr, "All data has been deleted. Database schema preserved.")
	return nil
}
