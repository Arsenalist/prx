package cmd

import (
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

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: ./prx.yaml)")
	rootCmd.PersistentFlags().StringVar(&dbPath, "db", "", "override database path")
	rootCmd.PersistentFlags().StringVar(&format, "format", "", "output format: table (default), json, markdown, all")
	rootCmd.PersistentFlags().BoolVar(&quiet, "quiet", false, "suppress non-essential output")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "verbose/debug output")
}
