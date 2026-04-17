package main

import (
	"os"

	"ssh-manager/cmd"
)

func main() {
	os.Exit(func() int {
		cmd.Execute()
		return 0
	}())
}
