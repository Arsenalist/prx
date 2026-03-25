package cmd

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage settings",
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value",
	Long: `Set a configuration value. Keys include:
  fetch.states           JSON array, e.g. '["closed","open"]'
  fetch.per_page         integer (default: 100)
  fetch.max_retries      integer (default: 3)
  fetch.rate_limit_buffer integer (default: 100)
  date_range.preset      last-7d, last-14d, last-30d, last-90d, this-week, etc.
  date_range.start       YYYY-MM-DD
  date_range.end         YYYY-MM-DD
  output.format          table, json, markdown, all
  output.directory       path for markdown reports
  output.timezone        e.g. America/Toronto
  test_patterns          JSON array of regex strings
  hooks                  JSON object`,
	Args: cobra.ExactArgs(2),
	RunE: runConfigSet,
}

var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Get a configuration value",
	Args:  cobra.ExactArgs(1),
	RunE:  runConfigGet,
}

var configListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all settings",
	RunE:  runConfigList,
}

var configResetCmd = &cobra.Command{
	Use:   "reset <key>",
	Short: "Remove a setting (revert to default)",
	Args:  cobra.ExactArgs(1),
	RunE:  runConfigReset,
}

func init() {
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configListCmd)
	configCmd.AddCommand(configResetCmd)
	rootCmd.AddCommand(configCmd)
}

func runConfigSet(cmd *cobra.Command, args []string) error {
	db, cleanup, err := openDB()
	if err != nil {
		return err
	}
	defer cleanup()

	key, value := args[0], args[1]
	if err := db.SetSetting(key, value); err != nil {
		return fmt.Errorf("setting config: %w", err)
	}

	fmt.Printf("%s = %s\n", key, value)
	return nil
}

func runConfigGet(cmd *cobra.Command, args []string) error {
	db, cleanup, err := openDB()
	if err != nil {
		return err
	}
	defer cleanup()

	key := args[0]
	value, err := db.GetSetting(key)
	if err != nil {
		return err
	}

	if value == "" {
		fmt.Printf("%s (not set, using default)\n", key)
	} else {
		fmt.Printf("%s = %s\n", key, value)
	}
	return nil
}

func runConfigList(cmd *cobra.Command, args []string) error {
	db, cleanup, err := openDB()
	if err != nil {
		return err
	}
	defer cleanup()

	settings, err := db.ListSettings()
	if err != nil {
		return err
	}

	if len(settings) == 0 {
		fmt.Println("No custom settings. All values use defaults.")
		fmt.Println("Use `prx config set <key> <value>` to customize.")
		return nil
	}

	keys := make([]string, 0, len(settings))
	for k := range settings {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		fmt.Printf("  %-30s %s\n", k, settings[k])
	}
	return nil
}

func runConfigReset(cmd *cobra.Command, args []string) error {
	db, cleanup, err := openDB()
	if err != nil {
		return err
	}
	defer cleanup()

	key := args[0]
	if err := db.DeleteSetting(key); err != nil {
		return fmt.Errorf("resetting config: %w", err)
	}

	fmt.Printf("%s reset to default.\n", key)
	return nil
}
