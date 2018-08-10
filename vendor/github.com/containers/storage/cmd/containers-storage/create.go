package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strconv"
	"strings"

	"github.com/containers/storage"
	"github.com/containers/storage/opts"
	"github.com/containers/storage/pkg/idtools"
	"github.com/containers/storage/pkg/mflag"
	digest "github.com/opencontainers/go-digest"
)

var (
	paramMountLabel   = ""
	paramNames        = []string{}
	paramID           = ""
	paramDigest       = ""
	paramLayer        = ""
	paramMetadata     = ""
	paramMetadataFile = ""
	paramCreateRO     = false
	paramHostUIDMap   = false
	paramHostGIDMap   = false
	paramUIDMap       = ""
	paramGIDMap       = ""
	paramSubUIDMap    = ""
	paramSubGIDMap    = ""
)

func paramIDMapping() (*storage.IDMappingOptions, error) {
	options := storage.IDMappingOptions{
		HostUIDMapping: paramHostUIDMap,
		HostGIDMapping: paramHostGIDMap,
	}
	if paramHostUIDMap && (len(paramUIDMap) > 0 || paramSubUIDMap != "") {
		return nil, fmt.Errorf("host uid map specified along with UID mapping")
	}
	if paramHostGIDMap && (len(paramGIDMap) > 0 || paramSubGIDMap != "") {
		return nil, fmt.Errorf("host gid map specified along with GID mapping")
	}
	if paramSubGIDMap == "" && paramSubUIDMap != "" {
		paramSubGIDMap = paramSubUIDMap
	}
	if paramSubUIDMap == "" && paramSubGIDMap != "" {
		paramSubUIDMap = paramSubGIDMap
	}
	nonDigitsToWhitespace := func(r rune) rune {
		if strings.IndexRune("0123456789", r) == -1 {
			return ' '
		} else {
			return r
		}
	}
	parseTriple := func(spec []string) (container, host, size uint32, err error) {
		cid, err := strconv.ParseUint(spec[0], 10, 32)
		if err != nil {
			return 0, 0, 0, fmt.Errorf("error parsing id map value %q: %v", spec[0], err)
		}
		hid, err := strconv.ParseUint(spec[1], 10, 32)
		if err != nil {
			return 0, 0, 0, fmt.Errorf("error parsing id map value %q: %v", spec[1], err)
		}
		sz, err := strconv.ParseUint(spec[2], 10, 32)
		if err != nil {
			return 0, 0, 0, fmt.Errorf("error parsing id map value %q: %v", spec[2], err)
		}
		return uint32(cid), uint32(hid), uint32(sz), nil
	}
	parseIDMap := func(idMapSpec, mapType string) (idmap []idtools.IDMap, err error) {
		if len(idMapSpec) > 0 {
			idSpec := strings.Fields(strings.Map(nonDigitsToWhitespace, idMapSpec))
			if len(idSpec)%3 != 0 {
				return nil, fmt.Errorf("%s map is malformed", mapType)
			}
			for i := range idSpec {
				if i%3 != 0 {
					continue
				}
				cid, hid, size, err := parseTriple(idSpec[i : i+3])
				if err != nil {
					return nil, fmt.Errorf("%s map is malformed", mapType)
				}
				mapping := idtools.IDMap{
					ContainerID: int(cid),
					HostID:      int(hid),
					Size:        int(size),
				}
				idmap = append(idmap, mapping)
			}
		}
		return idmap, nil
	}
	if paramSubUIDMap != "" && paramSubGIDMap != "" {
		mappings, err := idtools.NewIDMappings(paramSubUIDMap, paramSubGIDMap)
		if err != nil {
			return nil, err
		}
		options.UIDMap = mappings.UIDs()
		options.GIDMap = mappings.GIDs()
	}
	parsedUIDMap, err := parseIDMap(paramUIDMap, "uid")
	if err != nil {
		return nil, err
	}
	parsedGIDMap, err := parseIDMap(paramGIDMap, "gid")
	if err != nil {
		return nil, err
	}
	options.UIDMap = append(options.UIDMap, parsedUIDMap...)
	options.GIDMap = append(options.GIDMap, parsedGIDMap...)
	return &options, nil
}

func createLayer(flags *mflag.FlagSet, action string, m storage.Store, args []string) int {
	parent := ""
	if len(args) > 0 {
		parent = args[0]
	}
	mappings, err := paramIDMapping()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}
	options := &storage.LayerOptions{IDMappingOptions: *mappings}
	layer, err := m.CreateLayer(paramID, parent, paramNames, paramMountLabel, !paramCreateRO, options)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}
	if jsonOutput {
		json.NewEncoder(os.Stdout).Encode(layer)
	} else {
		fmt.Printf("%s\n", layer.ID)
		for _, name := range layer.Names {
			fmt.Printf("\t%s\n", name)
		}
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
	mappings, err := paramIDMapping()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}
	options := &storage.LayerOptions{IDMappingOptions: *mappings}
	layer, _, err := m.PutLayer(paramID, parent, paramNames, paramMountLabel, !paramCreateRO, options, diffStream)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}
	if jsonOutput {
		json.NewEncoder(os.Stdout).Encode(layer)
	} else {
		fmt.Printf("%s\n", layer.ID)
		for _, name := range layer.Names {
			fmt.Printf("\t%s\n", name)
		}
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
	layer := ""
	if len(args) > 0 {
		layer = args[0]
	}
	imageOptions := &storage.ImageOptions{
		Digest: digest.Digest(paramDigest),
	}
	image, err := m.CreateImage(paramID, paramNames, layer, paramMetadata, imageOptions)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}
	if jsonOutput {
		json.NewEncoder(os.Stdout).Encode(image)
	} else {
		fmt.Printf("%s\n", image.ID)
		for _, name := range image.Names {
			fmt.Printf("\t%s\n", name)
		}
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
	mappings, err := paramIDMapping()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}
	options := &storage.ContainerOptions{IDMappingOptions: *mappings}
	container, err := m.CreateContainer(paramID, paramNames, args[0], paramLayer, paramMetadata, options)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}
	if jsonOutput {
		json.NewEncoder(os.Stdout).Encode(container)
	} else {
		fmt.Printf("%s\n", container.ID)
		for _, name := range container.Names {
			fmt.Printf("\t%s", name)
		}
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
			flags.BoolVar(&paramHostUIDMap, []string{"-hostuidmap"}, paramHostUIDMap, "Force host UID map")
			flags.BoolVar(&paramHostGIDMap, []string{"-hostgidmap"}, paramHostGIDMap, "Force host GID map")
			flags.StringVar(&paramUIDMap, []string{"-uidmap"}, "", "UID map")
			flags.StringVar(&paramGIDMap, []string{"-gidmap"}, "", "GID map")
			flags.StringVar(&paramSubUIDMap, []string{"-subuidmap"}, "", "subuid UID map for a user")
			flags.StringVar(&paramSubGIDMap, []string{"-subgidmap"}, "", "subgid GID map for a group")
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
			flags.BoolVar(&paramHostUIDMap, []string{"-hostuidmap"}, paramHostUIDMap, "Force host UID map")
			flags.BoolVar(&paramHostGIDMap, []string{"-hostgidmap"}, paramHostGIDMap, "Force host GID map")
			flags.StringVar(&paramUIDMap, []string{"-uidmap"}, "", "UID map")
			flags.StringVar(&paramGIDMap, []string{"-gidmap"}, "", "GID map")
			flags.StringVar(&paramSubUIDMap, []string{"-subuidmap"}, "", "subuid UID map for a user")
			flags.StringVar(&paramSubGIDMap, []string{"-subgidmap"}, "", "subgid GID map for a group")
		},
	})
	commands = append(commands, command{
		names:       []string{"create-image", "createimage"},
		optionsHelp: "[options [...]] topLayerNameOrID",
		usage:       "Create a new image using layers",
		minArgs:     0,
		maxArgs:     1,
		action:      createImage,
		addFlags: func(flags *mflag.FlagSet, cmd *command) {
			flags.Var(opts.NewListOptsRef(&paramNames, nil), []string{"-name", "n"}, "Image name")
			flags.StringVar(&paramID, []string{"-id", "i"}, "", "Image ID")
			flags.StringVar(&paramDigest, []string{"-digest", "d"}, "", "Image Digest")
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
			flags.BoolVar(&paramHostUIDMap, []string{"-hostuidmap"}, paramHostUIDMap, "Force host UID map")
			flags.BoolVar(&paramHostGIDMap, []string{"-hostgidmap"}, paramHostGIDMap, "Force host GID map")
			flags.StringVar(&paramUIDMap, []string{"-uidmap"}, "", "UID map")
			flags.StringVar(&paramGIDMap, []string{"-gidmap"}, "", "GID map")
			flags.StringVar(&paramSubUIDMap, []string{"-subuidmap"}, "", "subuid UID map for a user")
			flags.StringVar(&paramSubGIDMap, []string{"-subgidmap"}, "", "subgid GID map for a group")
		},
	})
}
