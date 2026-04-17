package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var version = "1.0.0"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	Long:  `Show the version and build information of ssh-manager.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("ssh-manager version %s\n", version)
		fmt.Println("SSH Connection Manager for Linux")
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
