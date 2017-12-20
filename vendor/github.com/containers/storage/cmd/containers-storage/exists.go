package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/containers/storage"
	"github.com/containers/storage/pkg/mflag"
)

var (
	existLayer     = false
	existImage     = false
	existContainer = false
	existQuiet     = false
)

func exist(flags *mflag.FlagSet, action string, m storage.Store, args []string) int {
	if len(args) < 1 {
		return 1
	}
	anyMissing := false
	existDict := make(map[string]bool)
	for _, what := range args {
		exists := m.Exists(what)
		existDict[what] = exists
		if existContainer {
			if c, err := m.Container(what); c == nil || err != nil {
				exists = false
			}
		}
		if existImage {
			if i, err := m.Image(what); i == nil || err != nil {
				exists = false
			}
		}
		if existLayer {
			if l, err := m.Layer(what); l == nil || err != nil {
				exists = false
			}
		}
		if !exists {
			anyMissing = true
		}
	}
	if jsonOutput {
		json.NewEncoder(os.Stdout).Encode(existDict)
	} else {
		if !existQuiet {
			for what, exists := range existDict {
				fmt.Printf("%s: %v\n", what, exists)
			}
		}
	}
	if anyMissing {
		return 1
	}
	return 0
}

func init() {
	commands = append(commands, command{
		names:       []string{"exists"},
		optionsHelp: "[LayerOrImageOrContainerNameOrID [...]]",
		usage:       "Check if a layer or image or container exists",
		minArgs:     1,
		action:      exist,
		addFlags: func(flags *mflag.FlagSet, cmd *command) {
			flags.BoolVar(&existQuiet, []string{"-quiet", "q"}, existQuiet, "Don't print names")
			flags.BoolVar(&existLayer, []string{"-layer", "l"}, existQuiet, "Only succeed if the match is a layer")
			flags.BoolVar(&existImage, []string{"-image", "i"}, existQuiet, "Only succeed if the match is an image")
			flags.BoolVar(&existContainer, []string{"-container", "c"}, existQuiet, "Only succeed if the match is a container")
			flags.BoolVar(&jsonOutput, []string{"-json", "j"}, jsonOutput, "Prefer JSON output")
		},
	})
}
