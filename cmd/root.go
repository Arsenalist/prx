package cmd

import (
	"fmt"

	"github.com/Arsenalist/prx/internal/config"
	"github.com/spf13/cobra"
)

var (
	cfgFile string
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

// loadConfig resolves and loads the config file. Commands that need config should call this.
func loadConfig() (*config.Config, error) {
	path, source, err := config.Resolve(cfgFile)
	if err != nil {
		return nil, err
	}

	cfg, err := config.Load(path)
	if err != nil {
		return nil, err
	}

	if verbose {
		fmt.Printf("Config loaded from %s (%s)\n", path, source)
	}

	// CLI overrides
	if dbPath != "" {
		cfg.Storage.SQLite.Path = dbPath
	}
	if format != "" {
		cfg.Output.Format = format
	}

	return cfg, nil
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: ./prx.yaml)")
	rootCmd.PersistentFlags().StringVar(&dbPath, "db", "", "override database path")
	rootCmd.PersistentFlags().StringVar(&format, "format", "", "output format: table (default), json, markdown, all")
	rootCmd.PersistentFlags().BoolVar(&quiet, "quiet", false, "suppress non-essential output")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "verbose/debug output")
}
