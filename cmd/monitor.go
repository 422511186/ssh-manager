package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"ssh-manager/internal"
)

var monitorCmd = &cobra.Command{
	Use:   "monitor",
	Short: "Start interactive monitor panel",
	Long: `Start an interactive terminal UI to monitor SSH sessions in real-time.

Features:
  - Real-time session list
  - Auto-refresh every 5 seconds
  - Kill sessions with 'k' key
  - Ban IPs with 'b' key
  - Auto-kill idle sessions (if configured)

Controls:
  q - Quit
  r - Refresh
  k - Kill selected session
  b - Ban IP of selected session

Examples:
  ssh-manager monitor`,
	RunE: runMonitor,
}

func init() {
	rootCmd.AddCommand(monitorCmd)
}

func runMonitor(cmd *cobra.Command, args []string) error {
	cfg, err := internal.LoadConfig()
	if err != nil {
		cfg = &internal.Config{
			Monitor: internal.MonitorConfig{
				AutoKillIdle:   false,
				IdleThreshold:  30,
				RefreshSeconds: 5,
			},
		}
	}

	tui := internal.NewTUI()
	tui.SetConfig(&cfg.Monitor)

	fmt.Println("Starting SSH Monitor Panel...")
	fmt.Println("Press 'q' to quit, 'r' to refresh, 'k' to kill, 'b' to ban IP")
	fmt.Println()

	return tui.RunMonitor()
}
