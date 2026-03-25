package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

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
	RunE:  runDBQuery,
}

var dbPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Print the database file path",
	RunE:  runDBPath,
}

var dbStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show database size and table row counts",
	RunE:  runDBStats,
}

var dbRawCmd = &cobra.Command{
	Use:   "raw [repo] [pr-number]",
	Short: "Dump raw JSON blob for a specific PR",
	Args:  cobra.ExactArgs(2),
	RunE:  runDBRaw,
}

func init() {
	dbCmd.AddCommand(dbQueryCmd)
	dbCmd.AddCommand(dbPathCmd)
	dbCmd.AddCommand(dbStatsCmd)
	dbCmd.AddCommand(dbRawCmd)
	rootCmd.AddCommand(dbCmd)
}

func runDBPath(cmd *cobra.Command, args []string) error {
	fmt.Println(resolveDBPath())
	return nil
}

func runDBStats(cmd *cobra.Command, args []string) error {
	db, cleanup, err := openDB()
	if err != nil {
		return err
	}
	defer cleanup()

	path := resolveDBPath()
	info, err := os.Stat(path)
	if err == nil {
		fmt.Printf("Database: %s (%.1f MB)\n", path, float64(info.Size())/(1024*1024))
	}

	stats, err := db.Stats()
	if err != nil {
		return err
	}
	for table, count := range stats.Tables {
		fmt.Printf("  %-20s %d rows\n", table, count)
	}
	return nil
}

func runDBQuery(cmd *cobra.Command, args []string) error {
	db, cleanup, err := openDB()
	if err != nil {
		return err
	}
	defer cleanup()

	rows, err := db.RawQuery(args[0])
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

func runDBRaw(cmd *cobra.Command, args []string) error {
	db, cleanup, err := openDB()
	if err != nil {
		return err
	}
	defer cleanup()

	repoName := args[0]
	prNumber, err := strconv.Atoi(args[1])
	if err != nil {
		return fmt.Errorf("invalid PR number: %s", args[1])
	}

	// Find repo
	repos, err := db.ListRepositories()
	if err != nil {
		return err
	}

	var repoID int64
	for _, r := range repos {
		if r.FullName == repoName {
			repoID = r.ID
			break
		}
	}
	if repoID == 0 {
		return fmt.Errorf("repo %q not found in database", repoName)
	}

	// Query raw data
	rows, err := db.RawQuery(
		fmt.Sprintf("SELECT raw_data FROM pull_requests WHERE repo_id = %d AND number = %d", repoID, prNumber),
	)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return fmt.Errorf("PR #%d not found in %s", prNumber, repoName)
	}

	rawData, ok := rows[0]["raw_data"]
	if !ok || rawData == nil {
		fmt.Println("{}")
		return nil
	}

	// Pretty-print if it's valid JSON
	var parsed interface{}
	raw := fmt.Sprintf("%v", rawData)
	if err := json.Unmarshal([]byte(raw), &parsed); err == nil {
		pretty, _ := json.MarshalIndent(parsed, "", "  ")
		fmt.Println(string(pretty))
	} else {
		fmt.Println(raw)
	}
	return nil
}
