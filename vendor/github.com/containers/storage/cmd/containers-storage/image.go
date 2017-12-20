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
	paramImageDataFile = ""
)

func image(flags *mflag.FlagSet, action string, m storage.Store, args []string) int {
	matched := []*storage.Image{}
	for _, arg := range args {
		if image, err := m.Image(arg); err == nil {
			matched = append(matched, image)
		}
	}
	if jsonOutput {
		json.NewEncoder(os.Stdout).Encode(matched)
	} else {
		for _, image := range matched {
			fmt.Printf("ID: %s\n", image.ID)
			for _, name := range image.Names {
				fmt.Printf("Name: %s\n", name)
			}
			fmt.Printf("Top Layer: %s\n", image.TopLayer)
			for _, name := range image.BigDataNames {
				fmt.Printf("Data: %s\n", name)
			}
		}
	}
	if len(matched) != len(args) {
		return 1
	}
	return 0
}

func listImageBigData(flags *mflag.FlagSet, action string, m storage.Store, args []string) int {
	image, err := m.Image(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}
	d, err := m.ListImageBigData(image.ID)
	if jsonOutput {
		json.NewEncoder(os.Stdout).Encode(d)
	} else {
		for _, name := range d {
			fmt.Printf("%s\n", name)
		}
	}
	return 0
}

func getImageBigData(flags *mflag.FlagSet, action string, m storage.Store, args []string) int {
	image, err := m.Image(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}
	output := os.Stdout
	if paramImageDataFile != "" {
		f, err := os.Create(paramImageDataFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			return 1
		}
		output = f
	}
	b, err := m.ImageBigData(image.ID, args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}
	output.Write(b)
	output.Close()
	return 0
}

func setImageBigData(flags *mflag.FlagSet, action string, m storage.Store, args []string) int {
	image, err := m.Image(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}
	input := os.Stdin
	if paramImageDataFile != "" {
		f, err := os.Open(paramImageDataFile)
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
	err = m.SetImageBigData(image.ID, args[1], b)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}
	return 0
}

func init() {
	commands = append(commands,
		command{
			names:       []string{"image"},
			optionsHelp: "[options [...]] imageNameOrID [...]",
			usage:       "Examine an image",
			action:      image,
			minArgs:     1,
			addFlags: func(flags *mflag.FlagSet, cmd *command) {
				flags.BoolVar(&jsonOutput, []string{"-json", "j"}, jsonOutput, "Prefer JSON output")
			},
		},
		command{
			names:       []string{"list-image-data", "listimagedata"},
			optionsHelp: "[options [...]] imageNameOrID",
			usage:       "List data items that are attached to an image",
			action:      listImageBigData,
			minArgs:     1,
			maxArgs:     1,
			addFlags: func(flags *mflag.FlagSet, cmd *command) {
				flags.BoolVar(&jsonOutput, []string{"-json", "j"}, jsonOutput, "Prefer JSON output")
			},
		},
		command{
			names:       []string{"get-image-data", "getimagedata"},
			optionsHelp: "[options [...]] imageNameOrID dataName",
			usage:       "Get data that is attached to an image",
			action:      getImageBigData,
			minArgs:     2,
			addFlags: func(flags *mflag.FlagSet, cmd *command) {
				flags.StringVar(&paramImageDataFile, []string{"-file", "f"}, paramImageDataFile, "Write data to file")
			},
		},
		command{
			names:       []string{"set-image-data", "setimagedata"},
			optionsHelp: "[options [...]] imageNameOrID dataName",
			usage:       "Set data that is attached to an image",
			action:      setImageBigData,
			minArgs:     2,
			addFlags: func(flags *mflag.FlagSet, cmd *command) {
				flags.StringVar(&paramImageDataFile, []string{"-file", "f"}, paramImageDataFile, "Read data from file")
			},
		})
}
