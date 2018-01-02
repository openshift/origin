package main

import (
	"os"

	"github.com/containers/storage"
	"github.com/kubernetes-incubator/cri-o/libpod/images"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

type saveOptions struct {
	output string
	quiet  bool
	format string
	images []string
}

var (
	saveFlags = []cli.Flag{
		cli.StringFlag{
			Name:  "output, o",
			Usage: "Write to a file, default is STDOUT",
			Value: "/dev/stdout",
		},
		cli.BoolFlag{
			Name:  "quiet, q",
			Usage: "Suppress the output",
		},
		cli.StringFlag{
			Name:  "format",
			Usage: "Save image to oci-archive",
		},
	}
	saveDescription = `
	Save an image to docker-archive or oci-archive on the local machine.
	Default is docker-archive`

	saveCommand = cli.Command{
		Name:        "save",
		Usage:       "Save image to an archive",
		Description: saveDescription,
		Flags:       saveFlags,
		Action:      saveCmd,
		ArgsUsage:   "",
	}
)

// saveCmd saves the image to either docker-archive or oci
func saveCmd(c *cli.Context) error {
	args := c.Args()
	if len(args) == 0 {
		return errors.Errorf("need at least 1 argument")
	}

	config, err := getConfig(c)
	if err != nil {
		return errors.Wrapf(err, "could not get config")
	}
	store, err := getStore(config)
	if err != nil {
		return err
	}

	output := c.String("output")

	if output == "/dev/stdout" {
		fi := os.Stdout
		if logrus.IsTerminal(fi) {
			return errors.Errorf("refusing to save to terminal. Use -o flag or redirect")
		}
	}

	opts := saveOptions{
		output: output,
		quiet:  c.Bool("quiet"),
		format: c.String("format"),
		images: args,
	}

	return saveImage(store, opts)
}

// saveImage pushes the image to docker-archive or oci by
// calling pushImage
func saveImage(store storage.Store, opts saveOptions) error {
	var dst string
	switch opts.format {
	case images.OCIArchive:
		dst = images.OCIArchive + ":" + opts.output
	case images.DockerArchive:
		fallthrough
	case "":
		dst = images.DockerArchive + ":" + opts.output
	default:
		return errors.Errorf("unknown format option %q", opts.format)
	}

	saveOpts := images.CopyOptions{
		SignaturePolicyPath: "",
		Store:               store,
	}

	// only one image is supported for now
	// future pull requests will fix this
	for _, image := range opts.images {
		dest := dst + ":" + image
		if err := images.PushImage(image, dest, saveOpts); err != nil {
			return errors.Wrapf(err, "unable to save %q", image)
		}
	}
	return nil
}
