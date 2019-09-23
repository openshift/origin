package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/containers/storage"
	"github.com/containers/storage/pkg/mflag"
)

func containers(flags *mflag.FlagSet, action string, m storage.Store, args []string) int {
	containers, err := m.Containers()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		return 1
	}
	if jsonOutput {
		json.NewEncoder(os.Stdout).Encode(containers)
	} else {
		for _, container := range containers {
			fmt.Printf("%s\n", container.ID)
			for _, name := range container.Names {
				fmt.Printf("\tname: %s\n", name)
			}
			for _, name := range container.BigDataNames {
				fmt.Printf("\tdata: %s\n", name)
			}
		}
	}
	return 0
}

func init() {
	commands = append(commands, command{
		names:       []string{"containers"},
		optionsHelp: "[options [...]]",
		usage:       "List containers",
		action:      containers,
		maxArgs:     0,
		addFlags: func(flags *mflag.FlagSet, cmd *command) {
			flags.BoolVar(&jsonOutput, []string{"-json", "j"}, jsonOutput, "Prefer JSON output")
		},
	})
}
