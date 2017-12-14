package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/containers/storage"
	"github.com/containers/storage/pkg/mflag"
)

type mountPointOrError struct {
	ID         string `json:"id"`
	MountPoint string `json:"mountpoint"`
	Error      string `json:"error"`
}
type mountPointError struct {
	ID    string `json:"id"`
	Error string `json:"error"`
}

func mount(flags *mflag.FlagSet, action string, m storage.Store, args []string) int {
	moes := []mountPointOrError{}
	for _, arg := range args {
		result, err := m.Mount(arg, paramMountLabel)
		errText := ""
		if err != nil {
			errText = fmt.Sprintf("%v", err)
		}
		moes = append(moes, mountPointOrError{arg, result, errText})
	}
	if jsonOutput {
		json.NewEncoder(os.Stdout).Encode(moes)
	} else {
		for _, mountOrError := range moes {
			if mountOrError.Error != "" {
				fmt.Fprintf(os.Stderr, "%s while mounting %s\n", mountOrError.Error, mountOrError.ID)
			}
			fmt.Printf("%s\n", mountOrError.MountPoint)
		}
	}
	for _, mountOrErr := range moes {
		if mountOrErr.Error != "" {
			return 1
		}
	}
	return 0
}

func unmount(flags *mflag.FlagSet, action string, m storage.Store, args []string) int {
	mes := []mountPointError{}
	errors := false
	for _, arg := range args {
		err := m.Unmount(arg)
		errText := ""
		if err != nil {
			errText = fmt.Sprintf("%v", err)
			errors = true
		}
		mes = append(mes, mountPointError{arg, errText})
	}
	if jsonOutput {
		json.NewEncoder(os.Stdout).Encode(mes)
	} else {
		for _, me := range mes {
			if me.Error != "" {
				fmt.Fprintf(os.Stderr, "%s while unmounting %s\n", me.Error, me.ID)
			}
		}
	}
	if errors {
		return 1
	}
	return 0
}

func init() {
	commands = append(commands, command{
		names:       []string{"mount"},
		optionsHelp: "[options [...]] LayerOrContainerNameOrID",
		usage:       "Mount a layer or container",
		minArgs:     1,
		action:      mount,
		addFlags: func(flags *mflag.FlagSet, cmd *command) {
			flags.StringVar(&paramMountLabel, []string{"-label", "l"}, "", "Mount Label")
			flags.BoolVar(&jsonOutput, []string{"-json", "j"}, jsonOutput, "Prefer JSON output")
		},
	})
	commands = append(commands, command{
		names:       []string{"unmount", "umount"},
		optionsHelp: "LayerOrContainerNameOrID",
		usage:       "Unmount a layer or container",
		minArgs:     1,
		action:      unmount,
		addFlags: func(flags *mflag.FlagSet, cmd *command) {
			flags.BoolVar(&jsonOutput, []string{"-json", "j"}, jsonOutput, "Prefer JSON output")
		},
	})
}
