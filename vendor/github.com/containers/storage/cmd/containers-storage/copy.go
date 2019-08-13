package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/containers/storage"
	"github.com/containers/storage/pkg/chrootarchive"
	"github.com/containers/storage/pkg/idtools"
	"github.com/containers/storage/pkg/mflag"
)

var (
	chownOptions = ""
)

func copyContent(flags *mflag.FlagSet, action string, m storage.Store, args []string) int {
	var untarIDMappings *idtools.IDMappings
	var chownOpts *idtools.IDPair
	if len(args) < 1 {
		return 1
	}
	if len(chownOptions) > 0 {
		chownParts := strings.SplitN(chownOptions, ":", 2)
		if len(chownParts) == 1 {
			chownParts = append(chownParts, chownParts[0])
		}
		uid, err := strconv.ParseUint(chownParts[0], 10, 32)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error %q as a numeric UID: %v", chownParts[0], err)
			return 1
		}
		gid, err := strconv.ParseUint(chownParts[1], 10, 32)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error %q as a numeric GID: %v", chownParts[1], err)
			return 1
		}
		chownOpts = &idtools.IDPair{UID: int(uid), GID: int(gid)}
	}
	target := args[len(args)-1]
	if strings.Contains(target, ":") {
		targetParts := strings.SplitN(target, ":", 2)
		if len(targetParts) != 2 {
			fmt.Fprintf(os.Stderr, "error parsing target location %q: only one part\n", target)
			return 1
		}
		targetLayer, err := m.Layer(targetParts[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "error finding layer %q: %+v\n", targetParts[0], err)
			return 1
		}
		untarIDMappings = idtools.NewIDMappingsFromMaps(targetLayer.UIDMap, targetLayer.GIDMap)
		targetMount, err := m.Mount(targetLayer.ID, targetLayer.MountLabel)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error mounting layer %q: %+v\n", targetLayer.ID, err)
			return 1
		}
		target = filepath.Join(targetMount, targetParts[1])
		defer m.Unmount(targetLayer.ID, false)
	}
	args = args[:len(args)-1]
	for _, srcSpec := range args {
		var tarIDMappings *idtools.IDMappings
		source := srcSpec
		if strings.Contains(source, ":") {
			sourceParts := strings.SplitN(source, ":", 2)
			if len(sourceParts) != 2 {
				fmt.Fprintf(os.Stderr, "error parsing source location %q: only one part\n", source)
				return 1
			}
			sourceLayer, err := m.Layer(sourceParts[0])
			if err != nil {
				fmt.Fprintf(os.Stderr, "error finding layer %q: %+v\n", sourceParts[0], err)
				return 1
			}
			tarIDMappings = idtools.NewIDMappingsFromMaps(sourceLayer.UIDMap, sourceLayer.GIDMap)
			sourceMount, err := m.Mount(sourceLayer.ID, sourceLayer.MountLabel)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error mounting layer %q: %+v\n", sourceLayer.ID, err)
				return 1
			}
			source = filepath.Join(sourceMount, sourceParts[1])
			defer m.Unmount(sourceLayer.ID, false)
		}
		archiver := chrootarchive.NewArchiverWithChown(tarIDMappings, chownOpts, untarIDMappings)
		if err := archiver.CopyWithTar(source, target); err != nil {
			fmt.Fprintf(os.Stderr, "error copying %q to %q: %+v\n", source, target, err)
			return 1
		}
	}
	return 0
}

func init() {
	commands = append(commands, command{
		names:       []string{"copy"},
		usage:       "Copy files or directories into a layer, possibly from another layer",
		optionsHelp: "[options [...]] [sourceLayerNameOrID:]/path [...] targetLayerNameOrID:/path",
		minArgs:     2,
		action:      copyContent,
		addFlags: func(flags *mflag.FlagSet, cmd *command) {
			flags.StringVar(&chownOptions, []string{"-chown", ""}, chownOptions, "Set owner on new copies")
			flags.BoolVar(&jsonOutput, []string{"-json", "j"}, jsonOutput, "Prefer JSON output")
		},
	})
}
