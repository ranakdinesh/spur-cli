package main

import (
	"os"

	"github.com/ranakdinesh/spur-cli/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
