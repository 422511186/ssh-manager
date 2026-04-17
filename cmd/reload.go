package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"ssh-manager/internal"
)

var reloadCmd = &cobra.Command{
	Use:   "reload",
	Short: "Reload ban rules",
	Long: `Reload and apply the current ban rules from configuration.

This will:
  - Read the current ban rules from the configuration file
  - Update /etc/hosts.deny
  - Update iptables rules

Examples:
  ssh-manager reload`,
	RunE: runReload,
}

func init() {
	rootCmd.AddCommand(reloadCmd)
}

func runReload(cmd *cobra.Command, args []string) error {
	banner := internal.NewBanner()

	rules, err := banner.LoadCurrentRules()
	if err != nil {
		return fmt.Errorf("failed to load ban rules: %w", err)
	}

	fmt.Println("Reloading ban rules...")
	fmt.Printf("Users: %v\n", rules.Users)
	fmt.Printf("IPs: %v\n", rules.IPs)
	fmt.Printf("IP Ranges: %v\n", rules.Ranges)

	if err := banner.ApplyRules(rules); err != nil {
		return fmt.Errorf("failed to apply rules: %w", err)
	}

	fmt.Println("\nBan rules reloaded and applied successfully.")
	return nil
}
