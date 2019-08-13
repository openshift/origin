package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/containers/storage"
	"github.com/containers/storage/pkg/mflag"
	digest "github.com/opencontainers/go-digest"
)

var (
	imagesQuiet = false
)

func images(flags *mflag.FlagSet, action string, m storage.Store, args []string) int {
	images, err := m.Images()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		return 1
	}
	if jsonOutput {
		json.NewEncoder(os.Stdout).Encode(images)
	} else {
		for _, image := range images {
			fmt.Printf("%s\n", image.ID)
			if imagesQuiet {
				continue
			}
			for _, name := range image.Names {
				fmt.Printf("\tname: %s\n", name)
			}
			for _, digest := range image.Digests {
				fmt.Printf("\tdigest: %s\n", digest.String())
			}
			for _, name := range image.BigDataNames {
				fmt.Printf("\tdata: %s\n", name)
			}
		}
	}
	return 0
}

func imagesByDigest(flags *mflag.FlagSet, action string, m storage.Store, args []string) int {
	images := []*storage.Image{}
	for _, arg := range args {
		d := digest.Digest(arg)
		if err := d.Validate(); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", arg, err)
			return 1
		}
		matched, err := m.ImagesByDigest(d)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%+v\n", err)
			return 1
		}
		for _, match := range matched {
			images = append(images, match)
		}
	}
	if jsonOutput {
		json.NewEncoder(os.Stdout).Encode(images)
	} else {
		for _, image := range images {
			fmt.Printf("%s\n", image.ID)
			if imagesQuiet {
				continue
			}
			for _, name := range image.Names {
				fmt.Printf("\tname: %s\n", name)
			}
			for _, digest := range image.Digests {
				fmt.Printf("\tdigest: %s\n", digest.String())
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
			flags.BoolVar(&imagesQuiet, []string{"-quiet", "q"}, imagesQuiet, "Only print IDs")
		},
	})
	commands = append(commands, command{
		names:       []string{"images-by-digest"},
		optionsHelp: "[options [...]] DIGEST",
		usage:       "List images by digest",
		action:      imagesByDigest,
		minArgs:     1,
		maxArgs:     1,
		addFlags: func(flags *mflag.FlagSet, cmd *command) {
			flags.BoolVar(&jsonOutput, []string{"-json", "j"}, jsonOutput, "Prefer JSON output")
			flags.BoolVar(&imagesQuiet, []string{"-quiet", "q"}, imagesQuiet, "Only print IDs")
		},
	})
}
