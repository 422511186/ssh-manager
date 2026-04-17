package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	cfgFile string
	rootCmd = &cobra.Command{
		Use:   "ssh-manager",
		Short: "SSH Connection Manager for Linux",
		Long: `SSH-Manager is a CLI tool for managing SSH connections on Linux systems.

Features:
  - List active SSH sessions
  - Kill sessions by PID, user, IP, or TTY
  - Ban/unban users and IPs
  - Real-time monitor panel

Usage:
  ssh-manager [command] [flags]`,
		SilenceUsage: true,
	}
)

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file path")
}
