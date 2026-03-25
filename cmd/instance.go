package cmd

import (
	"fmt"

	"github.com/Arsenalist/prx/internal/store"
	"github.com/spf13/cobra"
)

var instanceCmd = &cobra.Command{
	Use:   "instance",
	Short: "Manage GitHub instances",
}

var instanceAddCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Add or update a GitHub instance",
	Args:  cobra.ExactArgs(1),
	RunE:  runInstanceAdd,
}

var instanceRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove an instance",
	Args:  cobra.ExactArgs(1),
	RunE:  runInstanceRemove,
}

var instanceListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all instances",
	RunE:  runInstanceList,
}

func init() {
	instanceAddCmd.Flags().String("url", "", "base URL (e.g., https://api.github.com)")
	instanceAddCmd.Flags().String("token-env", "", "environment variable name for the token")
	instanceAddCmd.Flags().String("type", "github", "instance type")
	instanceAddCmd.Flags().Bool("tls-skip-verify", false, "skip TLS certificate verification")
	instanceAddCmd.MarkFlagRequired("url")
	instanceAddCmd.MarkFlagRequired("token-env")

	instanceCmd.AddCommand(instanceAddCmd)
	instanceCmd.AddCommand(instanceRemoveCmd)
	instanceCmd.AddCommand(instanceListCmd)
	rootCmd.AddCommand(instanceCmd)
}

func runInstanceAdd(cmd *cobra.Command, args []string) error {
	db, cleanup, err := openDB()
	if err != nil {
		return err
	}
	defer cleanup()

	name := args[0]
	url, _ := cmd.Flags().GetString("url")
	tokenEnv, _ := cmd.Flags().GetString("token-env")
	instType, _ := cmd.Flags().GetString("type")
	tlsSkip, _ := cmd.Flags().GetBool("tls-skip-verify")

	_, err = db.UpsertInstance(store.InstanceRecord{
		Name:          name,
		Type:          instType,
		BaseURL:       url,
		TokenEnv:      tokenEnv,
		TLSSkipVerify: tlsSkip,
	})
	if err != nil {
		return fmt.Errorf("adding instance: %w", err)
	}

	fmt.Printf("Instance %q added.\n", name)
	return nil
}

func runInstanceRemove(cmd *cobra.Command, args []string) error {
	db, cleanup, err := openDB()
	if err != nil {
		return err
	}
	defer cleanup()

	name := args[0]
	if err := db.DeleteInstance(name); err != nil {
		return fmt.Errorf("removing instance: %w", err)
	}

	fmt.Printf("Instance %q removed.\n", name)
	return nil
}

func runInstanceList(cmd *cobra.Command, args []string) error {
	db, cleanup, err := openDB()
	if err != nil {
		return err
	}
	defer cleanup()

	instances, err := db.ListInstances()
	if err != nil {
		return err
	}

	if len(instances) == 0 {
		fmt.Println("No instances configured. Use `prx instance add` to add one.")
		return nil
	}

	for _, inst := range instances {
		tls := ""
		if inst.TLSSkipVerify {
			tls = " [tls-skip-verify]"
		}
		fmt.Printf("  %-20s %s (token: $%s)%s\n", inst.Name, inst.BaseURL, inst.TokenEnv, tls)
	}
	return nil
}
