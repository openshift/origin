package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/containers/storage"
	"github.com/containers/storage/opts"
	"github.com/containers/storage/pkg/mflag"
)

var (
	paramMountLabel   = ""
	paramNames        = []string{}
	paramID           = ""
	paramLayer        = ""
	paramMetadata     = ""
	paramMetadataFile = ""
	paramCreateRO     = false
)

func createLayer(flags *mflag.FlagSet, action string, m storage.Store, args []string) int {
	parent := ""
	if len(args) > 0 {
		parent = args[0]
	}
	layer, err := m.CreateLayer(paramID, parent, paramNames, paramMountLabel, !paramCreateRO)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}
	if jsonOutput {
		json.NewEncoder(os.Stdout).Encode(layer)
	} else {
		fmt.Printf("%s", layer.ID)
		for _, name := range layer.Names {
			fmt.Printf("\t%s\n", name)
		}
		fmt.Printf("\n")
	}
	return 0
}

func importLayer(flags *mflag.FlagSet, action string, m storage.Store, args []string) int {
	parent := ""
	if len(args) > 0 {
		parent = args[0]
	}
	diffStream := io.Reader(os.Stdin)
	if applyDiffFile != "" {
		f, err := os.Open(applyDiffFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			return 1
		}
		diffStream = f
		defer f.Close()
	}
	layer, _, err := m.PutLayer(paramID, parent, paramNames, paramMountLabel, !paramCreateRO, diffStream)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}
	if jsonOutput {
		json.NewEncoder(os.Stdout).Encode(layer)
	} else {
		fmt.Printf("%s", layer.ID)
		for _, name := range layer.Names {
			fmt.Printf("\t%s\n", name)
		}
		fmt.Printf("\n")
	}
	return 0
}

func createImage(flags *mflag.FlagSet, action string, m storage.Store, args []string) int {
	if paramMetadataFile != "" {
		f, err := os.Open(paramMetadataFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			return 1
		}
		b, err := ioutil.ReadAll(f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			return 1
		}
		paramMetadata = string(b)
	}
	image, err := m.CreateImage(paramID, paramNames, args[0], paramMetadata, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}
	if jsonOutput {
		json.NewEncoder(os.Stdout).Encode(image)
	} else {
		fmt.Printf("%s", image.ID)
		for _, name := range image.Names {
			fmt.Printf("\t%s\n", name)
		}
		fmt.Printf("\n")
	}
	return 0
}

func createContainer(flags *mflag.FlagSet, action string, m storage.Store, args []string) int {
	if paramMetadataFile != "" {
		f, err := os.Open(paramMetadataFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			return 1
		}
		b, err := ioutil.ReadAll(f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			return 1
		}
		paramMetadata = string(b)
	}
	container, err := m.CreateContainer(paramID, paramNames, args[0], paramLayer, paramMetadata, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}
	if jsonOutput {
		json.NewEncoder(os.Stdout).Encode(container)
	} else {
		fmt.Printf("%s", container.ID)
		for _, name := range container.Names {
			fmt.Printf("\t%s", name)
		}
		fmt.Printf("\n")
	}
	return 0
}

func init() {
	commands = append(commands, command{
		names:       []string{"create-layer", "createlayer"},
		optionsHelp: "[options [...]] [parentLayerNameOrID]",
		usage:       "Create a new layer",
		maxArgs:     1,
		action:      createLayer,
		addFlags: func(flags *mflag.FlagSet, cmd *command) {
			flags.StringVar(&paramMountLabel, []string{"-label", "l"}, "", "Mount Label")
			flags.Var(opts.NewListOptsRef(&paramNames, nil), []string{"-name", "n"}, "Layer name")
			flags.StringVar(&paramID, []string{"-id", "i"}, "", "Layer ID")
			flags.BoolVar(&paramCreateRO, []string{"-readonly", "r"}, false, "Mark as read-only")
			flags.BoolVar(&jsonOutput, []string{"-json", "j"}, jsonOutput, "Prefer JSON output")
		},
	})
	commands = append(commands, command{
		names:       []string{"import-layer", "importlayer"},
		optionsHelp: "[options [...]] [parentLayerNameOrID]",
		usage:       "Import a new layer",
		maxArgs:     1,
		action:      importLayer,
		addFlags: func(flags *mflag.FlagSet, cmd *command) {
			flags.StringVar(&paramMountLabel, []string{"-label", "l"}, "", "Mount Label")
			flags.Var(opts.NewListOptsRef(&paramNames, nil), []string{"-name", "n"}, "Layer name")
			flags.StringVar(&paramID, []string{"-id", "i"}, "", "Layer ID")
			flags.BoolVar(&paramCreateRO, []string{"-readonly", "r"}, false, "Mark as read-only")
			flags.BoolVar(&jsonOutput, []string{"-json", "j"}, jsonOutput, "Prefer JSON output")
			flags.StringVar(&applyDiffFile, []string{"-file", "f"}, "", "Read from file instead of stdin")
		},
	})
	commands = append(commands, command{
		names:       []string{"create-image", "createimage"},
		optionsHelp: "[options [...]] topLayerNameOrID",
		usage:       "Create a new image using layers",
		minArgs:     1,
		maxArgs:     1,
		action:      createImage,
		addFlags: func(flags *mflag.FlagSet, cmd *command) {
			flags.Var(opts.NewListOptsRef(&paramNames, nil), []string{"-name", "n"}, "Image name")
			flags.StringVar(&paramID, []string{"-id", "i"}, "", "Image ID")
			flags.StringVar(&paramMetadata, []string{"-metadata", "m"}, "", "Metadata")
			flags.StringVar(&paramMetadataFile, []string{"-metadata-file", "f"}, "", "Metadata File")
			flags.BoolVar(&jsonOutput, []string{"-json", "j"}, jsonOutput, "Prefer JSON output")
		},
	})
	commands = append(commands, command{
		names:       []string{"create-container", "createcontainer"},
		optionsHelp: "[options [...]] parentImageNameOrID",
		usage:       "Create a new container from an image",
		minArgs:     1,
		maxArgs:     1,
		action:      createContainer,
		addFlags: func(flags *mflag.FlagSet, cmd *command) {
			flags.Var(opts.NewListOptsRef(&paramNames, nil), []string{"-name", "n"}, "Container name")
			flags.StringVar(&paramID, []string{"-id", "i"}, "", "Container ID")
			flags.StringVar(&paramMetadata, []string{"-metadata", "m"}, "", "Metadata")
			flags.StringVar(&paramMetadataFile, []string{"-metadata-file", "f"}, "", "Metadata File")
			flags.BoolVar(&jsonOutput, []string{"-json", "j"}, jsonOutput, "Prefer JSON output")
		},
	})
}
