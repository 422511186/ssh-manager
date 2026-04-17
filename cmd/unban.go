package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"ssh-manager/internal"
)

var (
	unbanUsers  []string
	unbanIPs    []string
	unbanRanges []string
	unbanAll    bool
)

var unbanCmd = &cobra.Command{
	Use:   "unban",
	Short: "Unban users or IPs from SSH",
	Long: `Remove users or IP addresses from the ban list.

Can unban:
  - Specific users with --user flag
  - Specific IPs with --ip flag
  - IP ranges with --range flag
  - All banned entries with --all flag

Examples:
  ssh-manager unban --user hacker --ip 1.2.3.4
  ssh-manager unban --range 192.168.0.0/24
  ssh-manager unban --all`,
	RunE: runUnban,
}

func init() {
	unbanCmd.Flags().StringArrayVarP(&unbanUsers, "user", "u", []string{}, "User to unban")
	unbanCmd.Flags().StringArrayVarP(&unbanIPs, "ip", "i", []string{}, "IP address to unban")
	unbanCmd.Flags().StringArrayVarP(&unbanRanges, "range", "r", []string{}, "IP range to unban (CIDR)")
	unbanCmd.Flags().BoolVar(&unbanAll, "all", false, "Unban all entries")
	rootCmd.AddCommand(unbanCmd)
}

func runUnban(cmd *cobra.Command, args []string) error {
	banner := internal.NewBanner()

	rules, err := banner.LoadCurrentRules()
	if err != nil {
		rules = &internal.BanRules{}
	}

	if unbanAll {
		rules.Users = []string{}
		rules.IPs = []string{}
		rules.Ranges = []string{}
		fmt.Println("All ban entries cleared.")
	} else {
		for _, user := range unbanUsers {
			rules.Users = removeFromSlice(rules.Users, user)
			fmt.Printf("Removed user from ban list: %s\n", user)
		}

		for _, ip := range unbanIPs {
			rules.IPs = removeFromSlice(rules.IPs, ip)
			fmt.Printf("Removed IP from ban list: %s\n", ip)
		}

		for _, cidr := range unbanRanges {
			rules.Ranges = removeFromSlice(rules.Ranges, cidr)
			fmt.Printf("Removed IP range from ban list: %s\n", cidr)
		}
	}

	if err := banner.ApplyRules(rules); err != nil {
		return fmt.Errorf("failed to apply unban rules: %w", err)
	}

	fmt.Println("\nBan rules updated successfully.")
	showBanList(rules)

	return nil
}

func removeFromSlice(slice []string, item string) []string {
	result := []string{}
	for _, s := range slice {
		if s != item {
			result = append(result, s)
		}
	}
	return result
}
