package cmd

import (
	"fmt"
	"os"

	"github.com/Arsenalist/prx/internal/config"
	"github.com/Arsenalist/prx/internal/store/sqlite"
	"github.com/spf13/cobra"
)

var (
	dbPath  string
	format  string
	quiet   bool
	verbose bool
)

var rootCmd = &cobra.Command{
	Use:   "prx",
	Short: "PR analytics for engineering teams",
	Long:  `prx fetches pull request data from GitHub (including Enterprise), stores it locally, and generates developer productivity metrics, team reports, and agent-consumable structured output.`,
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

// resolveDBPath returns the database path from: --db flag > $PRX_DB > default.
func resolveDBPath() string {
	if dbPath != "" {
		return dbPath
	}
	if envDB := os.Getenv("PRX_DB"); envDB != "" {
		return envDB
	}
	return config.DefaultDBPath()
}

// openDB opens and migrates the database. Returns the store and a cleanup function.
func openDB() (*sqlite.SQLiteStore, func(), error) {
	path := resolveDBPath()
	db := sqlite.New(path)
	if err := db.Open(); err != nil {
		return nil, nil, fmt.Errorf("opening database: %w", err)
	}
	if err := db.Migrate(); err != nil {
		db.Close()
		return nil, nil, fmt.Errorf("migrating database: %w", err)
	}
	return db, func() { db.Close() }, nil
}

// loadSettings reads settings from DB and applies CLI overrides.
func loadSettings(db config.SettingsReader) (*config.Settings, error) {
	s, err := config.LoadSettings(db)
	if err != nil {
		return nil, err
	}
	if format != "" {
		s.Output.Format = format
	}
	return s, nil
}

func init() {
	rootCmd.PersistentFlags().StringVar(&dbPath, "db", "", "override database path")
	rootCmd.PersistentFlags().StringVar(&format, "format", "", "output format: table (default), json, markdown, all")
	rootCmd.PersistentFlags().BoolVar(&quiet, "quiet", false, "suppress non-essential output")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "verbose/debug output")
}
