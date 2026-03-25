package cmd

import (
	"fmt"
	"os"

	"github.com/Arsenalist/prx/internal/config"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Generate a starter prx.yaml config file",
	RunE:  runInit,
}

func init() {
	initCmd.Flags().String("path", "./prx.yaml", "where to create config")
	initCmd.Flags().Bool("minimal", false, "generate minimal config")
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	path, _ := cmd.Flags().GetString("path")
	minimal, _ := cmd.Flags().GetBool("minimal")

	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("config file already exists: %s (delete it first to regenerate)", path)
	}

	tmpl := config.FullTemplate
	if minimal {
		tmpl = config.MinimalTemplate
	}

	if err := os.WriteFile(path, []byte(tmpl), 0644); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}

	fmt.Printf("Created %s\n", path)
	return nil
}
