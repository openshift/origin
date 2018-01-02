package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/containers/storage"
	"github.com/containers/storage/pkg/mflag"
)

var listLayersTree = false

func layers(flags *mflag.FlagSet, action string, m storage.Store, args []string) int {
	layers, err := m.Layers()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}
	if jsonOutput {
		json.NewEncoder(os.Stdout).Encode(layers)
		return 0
	}
	imageMap := make(map[string]*[]storage.Image)
	if images, err := m.Images(); err == nil {
		for _, image := range images {
			if ilist, ok := imageMap[image.TopLayer]; ok && ilist != nil {
				list := append(*ilist, image)
				imageMap[image.TopLayer] = &list
			} else {
				list := []storage.Image{image}
				imageMap[image.TopLayer] = &list
			}
		}
	}
	containerMap := make(map[string]storage.Container)
	if containers, err := m.Containers(); err == nil {
		for _, container := range containers {
			containerMap[container.LayerID] = container
		}
	}
	nodes := []treeNode{}
	for _, layer := range layers {
		if listLayersTree {
			node := treeNode{
				left:  string(layer.Parent),
				right: string(layer.ID),
				notes: []string{},
			}
			if node.left == "" {
				node.left = "(base)"
			}
			for _, name := range layer.Names {
				node.notes = append(node.notes, "name: "+name)
			}
			if layer.MountPoint != "" {
				node.notes = append(node.notes, "mount: "+layer.MountPoint)
			}
			if imageList, ok := imageMap[layer.ID]; ok && imageList != nil {
				for _, image := range *imageList {
					node.notes = append(node.notes, fmt.Sprintf("image: %s", image.ID))
					for _, name := range image.Names {
						node.notes = append(node.notes, fmt.Sprintf("image name: %s", name))
					}
				}
			}
			if container, ok := containerMap[layer.ID]; ok {
				node.notes = append(node.notes, fmt.Sprintf("container: %s", container.ID))
				for _, name := range container.Names {
					node.notes = append(node.notes, fmt.Sprintf("container name: %s", name))
				}
			}
			nodes = append(nodes, node)
		} else {
			fmt.Printf("%s\n", layer.ID)
			for _, name := range layer.Names {
				fmt.Printf("\tname: %s\n", name)
			}
			if imageList, ok := imageMap[layer.ID]; ok && imageList != nil {
				for _, image := range *imageList {
					fmt.Printf("\timage: %s\n", image.ID)
					for _, name := range image.Names {
						fmt.Printf("\t\tname: %s\n", name)
					}
				}
			}
			if container, ok := containerMap[layer.ID]; ok {
				fmt.Printf("\tcontainer: %s\n", container.ID)
				for _, name := range container.Names {
					fmt.Printf("\t\tname: %s\n", name)
				}
			}
		}
	}
	if listLayersTree {
		printTree(nodes)
	}
	return 0
}

func init() {
	commands = append(commands, command{
		names:       []string{"layers"},
		optionsHelp: "[options [...]]",
		usage:       "List layers",
		action:      layers,
		maxArgs:     0,
		addFlags: func(flags *mflag.FlagSet, cmd *command) {
			flags.BoolVar(&listLayersTree, []string{"-tree", "t"}, listLayersTree, "Use a tree")
			flags.BoolVar(&jsonOutput, []string{"-json", "j"}, jsonOutput, "Prefer JSON output")
		},
	})
}
