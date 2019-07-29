package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/containers/storage"
	"github.com/containers/storage/pkg/mflag"
)

var (
	paramContainerDataFile = ""
)

func container(flags *mflag.FlagSet, action string, m storage.Store, args []string) int {
	images, err := m.Images()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		return 1
	}
	matches := []*storage.Container{}
	for _, arg := range args {
		if container, err := m.Container(arg); err == nil {
			matches = append(matches, container)
		}
	}
	if jsonOutput {
		json.NewEncoder(os.Stdout).Encode(matches)
	} else {
		for _, container := range matches {
			fmt.Printf("ID: %s\n", container.ID)
			for _, name := range container.Names {
				fmt.Printf("Name: %s\n", name)
			}
			fmt.Printf("Image: %s\n", container.ImageID)
			for _, image := range images {
				if image.ID == container.ImageID {
					for _, name := range image.Names {
						fmt.Printf("Image name: %s\n", name)
					}
					break
				}
			}
			size, err := m.ContainerSize(container.ID)
			if err != nil {
				fmt.Printf("Size unknown: %+v\n", err)
			} else {
				fmt.Printf("Size: %d\n", size)
			}
			fmt.Printf("Layer: %s\n", container.LayerID)
			for _, name := range container.BigDataNames {
				fmt.Printf("Data: %s\n", name)
			}
		}
	}
	if len(matches) != len(args) {
		return 1
	}
	return 0
}

func listContainerBigData(flags *mflag.FlagSet, action string, m storage.Store, args []string) int {
	container, err := m.Container(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		return 1
	}
	d, err := m.ListContainerBigData(container.ID)
	if jsonOutput {
		json.NewEncoder(os.Stdout).Encode(d)
	} else {
		for _, name := range d {
			fmt.Printf("%s\n", name)
		}
	}
	return 0
}

func getContainerBigData(flags *mflag.FlagSet, action string, m storage.Store, args []string) int {
	container, err := m.Container(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		return 1
	}
	output := os.Stdout
	if paramContainerDataFile != "" {
		f, err := os.Create(paramContainerDataFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			return 1
		}
		output = f
	}
	b, err := m.ContainerBigData(container.ID, args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		return 1
	}
	output.Write(b)
	output.Close()
	return 0
}

func getContainerBigDataSize(flags *mflag.FlagSet, action string, m storage.Store, args []string) int {
	container, err := m.Container(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		return 1
	}
	size, err := m.ContainerBigDataSize(container.ID, args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		return 1
	}
	fmt.Fprintf(os.Stdout, "%d\n", size)
	return 0
}

func getContainerBigDataDigest(flags *mflag.FlagSet, action string, m storage.Store, args []string) int {
	container, err := m.Container(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		return 1
	}
	d, err := m.ContainerBigDataDigest(container.ID, args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		return 1
	}
	if d.Validate() != nil {
		fmt.Fprintf(os.Stderr, "%v\n", d.Validate())
		return 1
	}
	fmt.Fprintf(os.Stdout, "%s\n", d.String())
	return 0
}

func setContainerBigData(flags *mflag.FlagSet, action string, m storage.Store, args []string) int {
	container, err := m.Container(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		return 1
	}
	input := os.Stdin
	if paramContainerDataFile != "" {
		f, err := os.Open(paramContainerDataFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			return 1
		}
		input = f
	}
	b, err := ioutil.ReadAll(input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}
	err = m.SetContainerBigData(container.ID, args[1], b)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		return 1
	}
	return 0
}

func getContainerDir(flags *mflag.FlagSet, action string, m storage.Store, args []string) int {
	path, err := m.ContainerDirectory(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		return 1
	}
	fmt.Printf("%s\n", path)
	return 0
}

func getContainerRunDir(flags *mflag.FlagSet, action string, m storage.Store, args []string) int {
	path, err := m.ContainerRunDirectory(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		return 1
	}
	fmt.Printf("%s\n", path)
	return 0
}

func containerParentOwners(flags *mflag.FlagSet, action string, m storage.Store, args []string) int {
	matched := []*storage.Container{}
	for _, arg := range args {
		if container, err := m.Container(arg); err == nil {
			matched = append(matched, container)
		}
	}
	for _, container := range matched {
		uids, gids, err := m.ContainerParentOwners(container.ID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ContainerParentOwner: %+v\n", err)
			return 1
		}
		if jsonOutput {
			mappings := struct {
				ID   string
				UIDs []int
				GIDs []int
			}{
				ID:   container.ID,
				UIDs: uids,
				GIDs: gids,
			}
			json.NewEncoder(os.Stdout).Encode(mappings)
		} else {
			fmt.Printf("ID: %s\n", container.ID)
			if len(uids) > 0 {
				fmt.Printf("UIDs: %v\n", uids)
			}
			if len(gids) > 0 {
				fmt.Printf("GIDs: %v\n", gids)
			}
		}
	}
	if len(matched) != len(args) {
		return 1
	}
	return 0
}

func init() {
	commands = append(commands,
		command{
			names:       []string{"container"},
			optionsHelp: "[options [...]] containerNameOrID [...]",
			usage:       "Examine a container",
			action:      container,
			minArgs:     1,
			addFlags: func(flags *mflag.FlagSet, cmd *command) {
				flags.BoolVar(&jsonOutput, []string{"-json", "j"}, jsonOutput, "Prefer JSON output")
			},
		},
		command{
			names:       []string{"list-container-data", "listcontainerdata"},
			optionsHelp: "[options [...]] containerNameOrID",
			usage:       "List data items that are attached to an container",
			action:      listContainerBigData,
			minArgs:     1,
			maxArgs:     1,
			addFlags: func(flags *mflag.FlagSet, cmd *command) {
				flags.BoolVar(&jsonOutput, []string{"-json", "j"}, jsonOutput, "Prefer JSON output")
			},
		},
		command{
			names:       []string{"get-container-data", "getcontainerdata"},
			optionsHelp: "[options [...]] containerNameOrID dataName",
			usage:       "Get data that is attached to an container",
			action:      getContainerBigData,
			minArgs:     2,
			addFlags: func(flags *mflag.FlagSet, cmd *command) {
				flags.StringVar(&paramContainerDataFile, []string{"-file", "f"}, paramContainerDataFile, "Write data to file")
			},
		},
		command{
			names:       []string{"get-container-data-size", "getcontainerdatasize"},
			optionsHelp: "[options [...]] containerNameOrID dataName",
			usage:       "Get size of data that is attached to an container",
			action:      getContainerBigDataSize,
			minArgs:     2,
		},
		command{
			names:       []string{"get-container-data-digest", "getcontainerdatadigest"},
			optionsHelp: "[options [...]] containerNameOrID dataName",
			usage:       "Get digest of data that is attached to an container",
			action:      getContainerBigDataDigest,
			minArgs:     2,
		},
		command{
			names:       []string{"set-container-data", "setcontainerdata"},
			optionsHelp: "[options [...]] containerNameOrID dataName",
			usage:       "Set data that is attached to an container",
			action:      setContainerBigData,
			minArgs:     2,
			addFlags: func(flags *mflag.FlagSet, cmd *command) {
				flags.StringVar(&paramContainerDataFile, []string{"-file", "f"}, paramContainerDataFile, "Read data from file")
			},
		},
		command{
			names:       []string{"get-container-dir", "getcontainerdir"},
			optionsHelp: "[options [...]] containerNameOrID",
			usage:       "Find the container's associated data directory",
			action:      getContainerDir,
			minArgs:     1,
		},
		command{
			names:       []string{"get-container-run-dir", "getcontainerrundir"},
			optionsHelp: "[options [...]] containerNameOrID",
			usage:       "Find the container's associated runtime directory",
			action:      getContainerRunDir,
			minArgs:     1,
		},
		command{
			names:       []string{"container-parent-owners"},
			optionsHelp: "[options [...]] containerNameOrID [...]",
			usage:       "Compute the set of unmapped parent UIDs and GIDs of the container",
			action:      containerParentOwners,
			minArgs:     1,
			addFlags: func(flags *mflag.FlagSet, cmd *command) {
				flags.BoolVar(&jsonOutput, []string{"-json", "j"}, jsonOutput, "Prefer JSON output")
			},
		})
}
