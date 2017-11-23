package main

import (
	"os"

	"fmt"
	"github.com/containers/image/docker/reference"
	"github.com/containers/image/pkg/sysregistries"
	"github.com/containers/image/transports/alltransports"
	"github.com/containers/image/types"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var (
	pullFlags = []cli.Flag{
		cli.BoolFlag{
			// all-tags is hidden since it has not been implemented yet
			Name:   "all-tags, a",
			Hidden: true,
			Usage:  "Download all tagged images in the repository",
		},
	}

	pullDescription = "Pulls an image from a registry and stores it locally.\n" +
		"An image can be pulled using its tag or digest. If a tag is not\n" +
		"specified, the image with the 'latest' tag (if it exists) is pulled."
	pullCommand = cli.Command{
		Name:        "pull",
		Usage:       "pull an image from a registry",
		Description: pullDescription,
		Flags:       pullFlags,
		Action:      pullCmd,
		ArgsUsage:   "",
	}
)

// struct for when a user passes a short or incomplete
// image name
type imagePullStruct struct {
	imageName   string
	tag         string
	registry    string
	hasRegistry bool
	transport   string
}

func (ips imagePullStruct) returnFQName() string {
	return fmt.Sprintf("%s%s/%s:%s", ips.transport, ips.registry, ips.imageName, ips.tag)
}

func getRegistriesToTry(image string) ([]string, error) {
	var registries []string
	var imageError = fmt.Sprintf("unable to parse '%s'\n", image)
	imgRef, err := reference.Parse(image)
	if err != nil {
		return nil, errors.Wrapf(err, imageError)
	}
	tagged, isTagged := imgRef.(reference.NamedTagged)
	tag := "latest"
	if isTagged {
		tag = tagged.Tag()
	}
	hasDomain := true
	registry := reference.Domain(imgRef.(reference.Named))
	if registry == "" {
		hasDomain = false
	}
	imageName := reference.Path(imgRef.(reference.Named))
	pImage := imagePullStruct{
		imageName,
		tag,
		registry,
		hasDomain,
		"docker://",
	}
	if pImage.hasRegistry {
		// If input has a registry, we have to assume they included an image
		// name but maybe not a tag
		pullRef, err := alltransports.ParseImageName(pImage.returnFQName())
		if err != nil {
			return nil, errors.Errorf(imageError)
		}
		registries = append(registries, pullRef.DockerReference().String())
	} else {
		// No registry means we check the globals registries configuration file
		// and assemble a list of candidate sources to try
		registryConfigPath := ""
		envOverride := os.Getenv("REGISTRIES_CONFIG_PATH")
		if len(envOverride) > 0 {
			registryConfigPath = envOverride
		}
		searchRegistries, err := sysregistries.GetRegistries(&types.SystemContext{SystemRegistriesConfPath: registryConfigPath})
		if err != nil {
			fmt.Println(err)
			return nil, errors.Errorf("unable to parse the registries.conf file and"+
				" the image name '%s' is incomplete.", imageName)
		}
		for _, searchRegistry := range searchRegistries {
			pImage.registry = searchRegistry
			pullRef, err := alltransports.ParseImageName(pImage.returnFQName())
			if err != nil {
				return nil, errors.Errorf("unable to parse '%s'", pImage.returnFQName())
			}
			registries = append(registries, pullRef.DockerReference().String())
		}
	}
	return registries, nil
}

// pullCmd gets the data from the command line and calls pullImage
// to copy an image from a registry to a local machine
func pullCmd(c *cli.Context) error {
	var fqRegistries []string

	args := c.Args()
	if len(args) == 0 {
		logrus.Errorf("an image name must be specified")
		return nil
	}
	if len(args) > 1 {
		logrus.Errorf("too many arguments. Requires exactly 1")
		return nil
	}
	image := args[0]
	srcRef, err := alltransports.ParseImageName(image)
	if err != nil {
		fqRegistries, err = getRegistriesToTry(image)
		if err != nil {
			fmt.Println(err)
		}
	} else {
		fqRegistries = append(fqRegistries, srcRef.DockerReference().String())
	}
	runtime, err := getRuntime(c)
	defer runtime.Shutdown(false)

	if err != nil {
		return errors.Wrapf(err, "could not create runtime")
	}
	for _, fqname := range fqRegistries {
		fmt.Printf("Trying to pull %s...", fqname)
		if err := runtime.PullImage(fqname, c.Bool("all-tags"), os.Stdout); err != nil {
			fmt.Printf(" Failed\n")
		} else {
			return nil
		}
	}
	return errors.Errorf("error pulling image from %q", image)
}
