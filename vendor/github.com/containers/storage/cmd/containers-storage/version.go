package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/containers/storage"
	"github.com/containers/storage/pkg/mflag"
)

func version(flags *mflag.FlagSet, action string, m storage.Store, args []string) int {
	version, err := m.Version()
	if err != nil {
		fmt.Fprintf(os.Stderr, "version: %v\n", err)
		return 1
	}
	if jsonOutput {
		json.NewEncoder(os.Stdout).Encode(version)
	} else {
		for _, pair := range version {
			fmt.Fprintf(os.Stderr, "%s: %s\n", pair[0], pair[1])
		}
	}
	return 0
}

func init() {
	commands = append(commands, command{
		names:   []string{"version"},
		usage:   "Return containers-storage version information",
		minArgs: 0,
		action:  version,
		addFlags: func(flags *mflag.FlagSet, cmd *command) {
			flags.BoolVar(&jsonOutput, []string{"-json", "j"}, jsonOutput, "Prefer JSON output")
		},
	})
}
