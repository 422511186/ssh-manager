package cmd

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"ssh-manager/internal"
)

var (
	killForce bool
)

var killCmd = &cobra.Command{
	Use:   "kill [pid|user|ip|tty]",
	Short: "Kill SSH sessions",
	Long: `Kill SSH sessions by specifying a target.

Target types:
  - PID: Kill a specific process by PID
  - user:<username>: Kill all sessions for a user
  - ip:<ip_address>: Kill all sessions from an IP
  - tty:<tty_device>: Kill a specific TTY session (e.g., pts/0)

Examples:
  ssh-manager kill 12345
  ssh-manager kill user:root
  ssh-manager kill ip:192.168.1.100
  ssh-manager kill tty:pts/0`,
	Args: cobra.ExactArgs(1),
	RunE: runKill,
}

func init() {
	killCmd.Flags().BoolVarP(&killForce, "force", "f", false, "Skip confirmation")
	rootCmd.AddCommand(killCmd)
}

func runKill(cmd *cobra.Command, args []string) error {
	target := args[0]
	killer := internal.NewKiller()

	switch {
	case isPID(target):
		return killByPID(target, killer)
	case hasPrefix(target, "user:"):
		return killByUser(trimPrefix(target, "user:"), killer)
	case hasPrefix(target, "ip:"):
		return killByIP(trimPrefix(target, "ip:"), killer)
	case hasPrefix(target, "tty:"):
		return killByTTY(trimPrefix(target, "tty:"), killer)
	default:
		return fmt.Errorf("invalid target format: %s", target)
	}
}

func isPID(s string) bool {
	_, err := strconv.Atoi(s)
	return err == nil
}

func hasPrefix(s, prefix string) bool {
	return len(s) > len(prefix) && s[:len(prefix)] == prefix
}

func trimPrefix(s, prefix string) string {
	if hasPrefix(s, prefix) {
		return s[len(prefix):]
	}
	return s
}

func killByPID(pidStr string, killer *internal.Killer) error {
	pid, _ := strconv.Atoi(pidStr)

	if !killForce {
		fmt.Printf("Are you sure you want to kill process %d? (y/N): ", pid)
		if !confirm() {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	if err := killer.KillByPID(pid); err != nil {
		return fmt.Errorf("failed to kill process %d: %w", pid, err)
	}

	fmt.Printf("Process %d killed successfully.\n", pid)
	return nil
}

func killByUser(user string, killer *internal.Killer) error {
	if !killForce {
		fmt.Printf("Are you sure you want to kill all sessions for user '%s'? (y/N): ", user)
		if !confirm() {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	killed, err := killer.KillByUser(user)
	if err != nil {
		return fmt.Errorf("failed to kill sessions: %w", err)
	}

	if len(killed) == 0 {
		fmt.Printf("No active sessions found for user '%s'.\n", user)
		return nil
	}

	fmt.Printf("Killed %d session(s) for user '%s'.\n", len(killed), user)
	return nil
}

func killByIP(ip string, killer *internal.Killer) error {
	if !killForce {
		fmt.Printf("Are you sure you want to kill all sessions from IP '%s'? (y/N): ", ip)
		if !confirm() {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	killed, err := killer.KillByIP(ip)
	if err != nil {
		return fmt.Errorf("failed to kill sessions: %w", err)
	}

	if len(killed) == 0 {
		fmt.Printf("No active sessions found from IP '%s'.\n", ip)
		return nil
	}

	fmt.Printf("Killed %d session(s) from IP '%s'.\n", len(killed), ip)
	return nil
}

func killByTTY(tty string, killer *internal.Killer) error {
	if !killForce {
		fmt.Printf("Are you sure you want to kill session on %s? (y/N): ", tty)
		if !confirm() {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	if err := killer.KillByTTY(tty); err != nil {
		return fmt.Errorf("failed to kill session: %w", err)
	}

	fmt.Printf("Session on %s killed successfully.\n", tty)
	return nil
}

func confirm() bool {
	var input string
	fmt.Scanln(&input)
	input = string([]byte(input))
	return input == "y" || input == "Y"
}
