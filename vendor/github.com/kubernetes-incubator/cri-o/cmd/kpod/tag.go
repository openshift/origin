package main

import (
	"github.com/containers/image/docker/reference"
	"github.com/containers/storage"
	"github.com/kubernetes-incubator/cri-o/libpod/images"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

var (
	tagDescription = "Adds one or more additional names to locally-stored image"
	tagCommand     = cli.Command{
		Name:        "tag",
		Usage:       "Add an additional name to a local image",
		Description: tagDescription,
		Action:      tagCmd,
		ArgsUsage:   "IMAGE-NAME [IMAGE-NAME ...]",
	}
)

func tagCmd(c *cli.Context) error {
	args := c.Args()
	if len(args) < 2 {
		return errors.Errorf("image name and at least one new name must be specified")
	}
	config, err := getConfig(c)
	if err != nil {
		return errors.Wrapf(err, "Could not get config")
	}
	store, err := getStore(config)
	if err != nil {
		return err
	}
	img, err := images.FindImage(store, args[0])
	if err != nil {
		return err
	}
	if img == nil {
		return errors.New("null image")
	}
	err = addImageNames(store, img, args[1:])
	if err != nil {
		return errors.Wrapf(err, "error adding names %v to image %q", args[1:], args[0])
	}
	return nil
}

func addImageNames(store storage.Store, image *storage.Image, addNames []string) error {
	// Add tags to the names if applicable
	names, err := expandedTags(addNames)
	if err != nil {
		return err
	}
	err = store.SetNames(image.ID, append(image.Names, names...))
	if err != nil {
		return errors.Wrapf(err, "error adding names (%v) to image %q", names, image.ID)
	}
	return nil
}

func expandedTags(tags []string) ([]string, error) {
	expandedNames := []string{}
	for _, tag := range tags {
		var labelName string
		name, err := reference.Parse(tag)
		if err != nil {
			return nil, errors.Wrapf(err, "error parsing tag %q", name)
		}
		if _, ok := name.(reference.NamedTagged); ok {
			labelName = name.String()
		} else {
			labelName = name.String() + ":latest"
		}
		expandedNames = append(expandedNames, labelName)
	}
	return expandedNames, nil
}
