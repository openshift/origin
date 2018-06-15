package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/containers/storage"
	"github.com/containers/storage/pkg/mflag"
)

func images(flags *mflag.FlagSet, action string, m storage.Store, args []string) int {
	images, err := m.Images()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}
	if jsonOutput {
		json.NewEncoder(os.Stdout).Encode(images)
	} else {
		for _, image := range images {
			fmt.Printf("%s\n", image.ID)
			for _, name := range image.Names {
				fmt.Printf("\tname: %s\n", name)
			}
			for _, name := range image.BigDataNames {
				fmt.Printf("\tdata: %s\n", name)
			}
		}
	}
	return 0
}

func init() {
	commands = append(commands, command{
		names:       []string{"images"},
		optionsHelp: "[options [...]]",
		usage:       "List images",
		action:      images,
		maxArgs:     0,
		addFlags: func(flags *mflag.FlagSet, cmd *command) {
			flags.BoolVar(&jsonOutput, []string{"-json", "j"}, jsonOutput, "Prefer JSON output")
		},
	})
}
