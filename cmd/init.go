package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize prx database",
	RunE:  runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	path := resolveDBPath()

	// Create parent directory if needed
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}

	db, cleanup, err := openDB()
	if err != nil {
		return err
	}
	defer cleanup()
	_ = db // migration already happened in openDB

	fmt.Printf("Database created at %s\n\n", path)
	fmt.Println("Get started:")
	fmt.Println("  prx instance add github --url https://api.github.com --token-env GITHUB_TOKEN")
	fmt.Println("  prx repo add owner/repo --instance github")
	fmt.Println("  prx fetch")
	return nil
}
