package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/containers/storage"
	"github.com/containers/storage/pkg/mflag"
)

var testDeleteImage = false

func deleteThing(flags *mflag.FlagSet, action string, m storage.Store, args []string) int {
	if len(args) < 1 {
		return 1
	}
	deleted := make(map[string]string)
	for _, what := range args {
		err := m.Delete(what)
		if err != nil {
			deleted[what] = fmt.Sprintf("%v", err)
		} else {
			deleted[what] = ""
		}
	}
	if jsonOutput {
		json.NewEncoder(os.Stdout).Encode(deleted)
	} else {
		for what, err := range deleted {
			if err != "" {
				fmt.Fprintf(os.Stderr, "%s: %s\n", what, err)
			}
		}
	}
	for _, err := range deleted {
		if err != "" {
			return 1
		}
	}
	return 0
}

func deleteLayer(flags *mflag.FlagSet, action string, m storage.Store, args []string) int {
	if len(args) < 1 {
		return 1
	}
	deleted := make(map[string]string)
	for _, what := range args {
		err := m.DeleteLayer(what)
		if err != nil {
			deleted[what] = fmt.Sprintf("%v", err)
		} else {
			deleted[what] = ""
		}
	}
	if jsonOutput {
		json.NewEncoder(os.Stdout).Encode(deleted)
	} else {
		for what, err := range deleted {
			if err != "" {
				fmt.Fprintf(os.Stderr, "%s: %s\n", what, err)
			}
		}
	}
	for _, err := range deleted {
		if err != "" {
			return 1
		}
	}
	return 0
}

type deletedImage struct {
	DeletedLayers []string `json:"deleted-layers,omitifempty"`
	Error         string   `json:"error,omitifempty"`
}

func deleteImage(flags *mflag.FlagSet, action string, m storage.Store, args []string) int {
	if len(args) < 1 {
		return 1
	}
	deleted := make(map[string]deletedImage)
	for _, what := range args {
		layers, err := m.DeleteImage(what, !testDeleteImage)
		errText := ""
		if err != nil {
			errText = fmt.Sprintf("%v", err)
		}
		deleted[what] = deletedImage{
			DeletedLayers: layers,
			Error:         errText,
		}
	}
	if jsonOutput {
		json.NewEncoder(os.Stdout).Encode(deleted)
	} else {
		for what, record := range deleted {
			if record.Error != "" {
				fmt.Fprintf(os.Stderr, "%s: %s\n", what, record.Error)
			} else {
				for _, layer := range record.DeletedLayers {
					fmt.Fprintf(os.Stderr, "%s: %s\n", what, layer)
				}
			}
		}
	}
	for _, record := range deleted {
		if record.Error != "" {
			return 1
		}
	}
	return 0
}

func deleteContainer(flags *mflag.FlagSet, action string, m storage.Store, args []string) int {
	if len(args) < 1 {
		return 1
	}
	deleted := make(map[string]string)
	for _, what := range args {
		err := m.DeleteContainer(what)
		if err != nil {
			deleted[what] = fmt.Sprintf("%v", err)
		} else {
			deleted[what] = ""
		}
	}
	if jsonOutput {
		json.NewEncoder(os.Stdout).Encode(deleted)
	} else {
		for what, err := range deleted {
			if err != "" {
				fmt.Fprintf(os.Stderr, "%s: %s\n", what, err)
			}
		}
	}
	for _, err := range deleted {
		if err != "" {
			return 1
		}
	}
	return 0
}

func init() {
	commands = append(commands, command{
		names:       []string{"delete"},
		optionsHelp: "[LayerOrImageOrContainerNameOrID [...]]",
		usage:       "Delete a layer or image or container, with no safety checks",
		minArgs:     1,
		action:      deleteThing,
		addFlags: func(flags *mflag.FlagSet, cmd *command) {
			flags.BoolVar(&jsonOutput, []string{"-json", "j"}, jsonOutput, "Prefer JSON output")
		},
	})
	commands = append(commands, command{
		names:       []string{"delete-layer", "deletelayer"},
		optionsHelp: "[LayerNameOrID [...]]",
		usage:       "Delete a layer, with safety checks",
		minArgs:     1,
		action:      deleteLayer,
		addFlags: func(flags *mflag.FlagSet, cmd *command) {
			flags.BoolVar(&jsonOutput, []string{"-json", "j"}, jsonOutput, "Prefer JSON output")
		},
	})
	commands = append(commands, command{
		names:       []string{"delete-image", "deleteimage"},
		optionsHelp: "[ImageNameOrID [...]]",
		usage:       "Delete an image, with safety checks",
		minArgs:     1,
		action:      deleteImage,
		addFlags: func(flags *mflag.FlagSet, cmd *command) {
			flags.BoolVar(&testDeleteImage, []string{"-test", "t"}, jsonOutput, "Only test removal")
			flags.BoolVar(&jsonOutput, []string{"-json", "j"}, jsonOutput, "Prefer JSON output")
		},
	})
	commands = append(commands, command{
		names:       []string{"delete-container", "deletecontainer"},
		optionsHelp: "[ContainerNameOrID [...]]",
		usage:       "Delete a container, with safety checks",
		minArgs:     1,
		action:      deleteContainer,
		addFlags: func(flags *mflag.FlagSet, cmd *command) {
			flags.BoolVar(&jsonOutput, []string{"-json", "j"}, jsonOutput, "Prefer JSON output")
		},
	})
}
