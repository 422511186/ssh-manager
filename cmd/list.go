package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"ssh-manager/internal"
)

var (
	listOutput  string
	listIdle    int
	listUser    string
	listUseTUI  bool
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List active SSH sessions",
	Long: `List all active SSH sessions connected to this machine.

Shows detailed information about each session including:
  - PID: sshd process ID
  - USER: Username
  - TTY: Pseudo-terminal device
  - IP: Remote connection IP
  - LOGIN: Login time
  - IDLE: Idle time

Examples:
  ssh-manager list
  ssh-manager list -o json
  ssh-manager list -i 30
  ssh-manager list -u root
  ssh-manager list --tui`,
	RunE: runList,
}

func init() {
	listCmd.Flags().StringVarP(&listOutput, "output", "o", "table", "Output format: table, json")
	listCmd.Flags().IntVarP(&listIdle, "idle", "i", 0, "Show sessions idle for more than N minutes")
	listCmd.Flags().StringVarP(&listUser, "user", "u", "", "Show sessions for specific user")
	listCmd.Flags().BoolVarP(&listUseTUI, "tui", "t", false, "Use interactive TUI")
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) error {
	scanner := internal.NewScanner()
	sessions, err := scanner.ScanSessions()
	if err != nil {
		return fmt.Errorf("failed to scan sessions: %w", err)
	}

	tunnels, _ := scanner.ScanTunnels()

	var filtered []internal.SSHSession
	for _, s := range sessions {
		if listUser != "" && s.User != listUser {
			continue
		}
		if listIdle > 0 && int(s.IdleTime.Minutes()) < listIdle {
			continue
		}
		filtered = append(filtered, s)
	}

	if listUseTUI {
		return runListTUI(filtered)
	}

	if listOutput == "json" {
		return printJSON(filtered, tunnels)
	}

	return printTable(filtered, tunnels)
}

func runListTUI(sessions []internal.SSHSession) error {
	tui := internal.NewTUI()
	tui.Sessions = sessions
	return tui.RunList()
}

func printJSON(sessions []internal.SSHSession, tunnels []internal.SSHTunnel) error {
	type SessionJSON struct {
		PID       int      `json:"pid"`
		User      string   `json:"user"`
		TTY       string   `json:"tty"`
		IP        string   `json:"ip"`
		LoginTime string   `json:"login_time"`
		IdleTime  string   `json:"idle_time"`
		Forwards  []string `json:"forwards,omitempty"`
	}

	var jsonSessions []SessionJSON
	for _, s := range sessions {
		var forwards []string
		for _, f := range s.Forwards {
			forwards = append(forwards, f.String())
		}
		jsonSessions = append(jsonSessions, SessionJSON{
			PID:       s.PID,
			User:      s.User,
			TTY:       s.TTY,
			IP:        s.IP,
			LoginTime: s.LoginTime.Format(time.RFC3339),
			IdleTime:  s.IdleTime.String(),
			Forwards:  forwards,
		})
	}

	type TunnelJSON struct {
		PID           int      `json:"pid"`
		User          string   `json:"user"`
		IP            string   `json:"ip"`
		LastActivity  string   `json:"last_activity"`
		Forwards     []string `json:"forwards"`
	}

	var jsonTunnels []TunnelJSON
	for _, t := range tunnels {
		var forwards []string
		for _, f := range t.Forwards {
			forwards = append(forwards, f.String())
		}
		lastActivity := ""
		if !t.LastActivity.IsZero() {
			lastActivity = t.LastActivity.Format(time.RFC3339)
		}
		jsonTunnels = append(jsonTunnels, TunnelJSON{
			PID:          t.PID,
			User:         t.User,
			IP:           t.IP,
			LastActivity: lastActivity,
			Forwards:    forwards,
		})
	}

	type Output struct {
		Sessions []SessionJSON  `json:"sessions"`
		Tunnels  []TunnelJSON  `json:"tunnels"`
	}

	data, err := json.MarshalIndent(Output{Sessions: jsonSessions, Tunnels: jsonTunnels}, "", "  ")
	if err != nil {
		return err
	}

	fmt.Println(string(data))
	return nil
}

func printTable(sessions []internal.SSHSession, tunnels []internal.SSHTunnel) error {
	if len(sessions) == 0 && len(tunnels) == 0 {
		fmt.Println("No active SSH sessions found.")
		return nil
	}

	header := fmt.Sprintf("%-6s %-10s %-8s %-15s %-20s %-10s",
		"PID", "USER", "TTY", "IP", "LOGIN TIME", "IDLE")
	separator := strings.Repeat("-", len(header))

	fmt.Println(header)
	fmt.Println(separator)

	for _, s := range sessions {
		idleColor := ""
		if s.IdleTime > 30*time.Minute {
			idleColor = "\033[31m"
		} else if s.IdleTime > 10*time.Minute {
			idleColor = "\033[33m"
		}

		fmt.Printf("%-6d %-10s %-8s %-15s %-20s %s%s\033[0m\n",
			s.PID, s.User, s.TTY, s.IP, s.LoginTime.Format("2006-01-02 15:04:05"), idleColor, s.GetDisplayIdle())

		if len(s.Forwards) > 0 {
			for _, f := range s.Forwards {
				fmt.Printf("         \\__ %s\n", f.String())
			}
		}
	}

	if len(tunnels) > 0 {
		fmt.Println(strings.Repeat("-", len(header)))
		fmt.Println("SSH Tunnels (no TTY):")

		for _, t := range tunnels {
			lastActivity := ""
			if !t.LastActivity.IsZero() {
				lastActivity = t.LastActivity.Format("2006-01-02 15:04:05")
			}
			fmt.Printf("%-6d %-10s %-8s %-15s %-20s\n", t.PID, t.User, "-", t.IP, lastActivity)
			for _, f := range t.Forwards {
				fmt.Printf("         \\__ %s\n", f.String())
			}
		}
	}

	fmt.Println(separator)
	fmt.Printf("Total: %d sessions, %d tunnels\n", len(sessions), len(tunnels))

	return nil
}
