package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"ssh-manager/internal"
)

var (
	banUsers    []string
	banIPs      []string
	banRanges   []string
	banAllUsers bool
	banAllIPs   bool
)

var banCmd = &cobra.Command{
	Use:   "ban",
	Short: "Ban users or IPs from SSH",
	Long: `Ban users or IP addresses from connecting via SSH.

Can ban:
  - Specific users with --user flag
  - Specific IPs with --ip flag
  - IP ranges with --range flag

Examples:
  ssh-manager ban --user hacker --ip 1.2.3.4
  ssh-manager ban --range 192.168.0.0/24
  ssh-manager ban --all-users`,
	RunE: runBan,
}

func init() {
	banCmd.Flags().StringArrayVarP(&banUsers, "user", "u", []string{}, "User to ban")
	banCmd.Flags().StringArrayVarP(&banIPs, "ip", "i", []string{}, "IP address to ban")
	banCmd.Flags().StringArrayVarP(&banRanges, "range", "r", []string{}, "IP range to ban (CIDR)")
	banCmd.Flags().BoolVar(&banAllUsers, "all-users", false, "Ban all currently connected users")
	banCmd.Flags().BoolVar(&banAllIPs, "all-ips", false, "Ban all currently connected IPs")
	rootCmd.AddCommand(banCmd)
}

func runBan(cmd *cobra.Command, args []string) error {
	banner := internal.NewBanner()
	_, err := internal.LoadConfig()

	rules, err := banner.LoadCurrentRules()
	if err != nil {
		rules = &internal.BanRules{}
	}

	if banAllUsers || banAllIPs {
		scanner := internal.NewScanner()
		sessions, err := scanner.ScanSessions()
		if err != nil {
			return fmt.Errorf("failed to scan sessions: %w", err)
		}

		userMap := make(map[string]bool)
		ipMap := make(map[string]bool)

		for _, s := range sessions {
			userMap[s.User] = true
			if s.IP != "" {
				ipMap[s.IP] = true
			}
		}

		if banAllUsers {
			for user := range userMap {
				if !contains(rules.Users, user) {
					rules.Users = append(rules.Users, user)
					fmt.Printf("Added user to ban list: %s\n", user)
				}
			}
		}

		if banAllIPs {
			for ip := range ipMap {
				if !contains(rules.IPs, ip) {
					rules.IPs = append(rules.IPs, ip)
					fmt.Printf("Added IP to ban list: %s\n", ip)
				}
			}
		}
	}

	for _, user := range banUsers {
		if !contains(rules.Users, user) {
			rules.Users = append(rules.Users, user)
			fmt.Printf("Added user to ban list: %s\n", user)
		} else {
			fmt.Printf("User already banned: %s\n", user)
		}
	}

	for _, ip := range banIPs {
		if !contains(rules.IPs, ip) && !contains(rules.Ranges, ip) {
			rules.IPs = append(rules.IPs, ip)
			fmt.Printf("Added IP to ban list: %s\n", ip)
		} else {
			fmt.Printf("IP already banned: %s\n", ip)
		}
	}

	for _, cidr := range banRanges {
		if !contains(rules.Ranges, cidr) && !contains(rules.IPs, cidr) {
			rules.Ranges = append(rules.Ranges, cidr)
			fmt.Printf("Added IP range to ban list: %s\n", cidr)
		} else {
			fmt.Printf("IP range already banned: %s\n", cidr)
		}
	}

	if err := banner.ApplyRules(rules); err != nil {
		return fmt.Errorf("failed to apply ban rules: %w", err)
	}

	fmt.Println("\nBan rules applied successfully.")
	showBanList(rules)

	return nil
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func showBanList(rules *internal.BanRules) {
	fmt.Println("\nCurrent ban list:")
	fmt.Println("Users:", rules.Users)
	fmt.Println("IPs:", rules.IPs)
	fmt.Println("IP Ranges:", rules.Ranges)
}
