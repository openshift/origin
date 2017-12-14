package main

import (
	"reflect"
	"strings"

	"github.com/containers/storage"
	"github.com/kubernetes-incubator/cri-o/cmd/kpod/formats"
	libpod "github.com/kubernetes-incubator/cri-o/libpod/images"
	digest "github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

var (
	imagesFlags = []cli.Flag{
		cli.BoolFlag{
			Name:  "quiet, q",
			Usage: "display only image IDs",
		},
		cli.BoolFlag{
			Name:  "noheading, n",
			Usage: "do not print column headings",
		},
		cli.BoolFlag{
			Name:  "no-trunc, notruncate",
			Usage: "do not truncate output",
		},
		cli.BoolFlag{
			Name:  "digests",
			Usage: "show digests",
		},
		cli.StringFlag{
			Name:  "format",
			Usage: "Change the output format to JSON or a Go template",
		},
		cli.StringFlag{
			Name:  "filter, f",
			Usage: "filter output based on conditions provided (default [])",
		},
	}

	imagesDescription = "lists locally stored images."
	imagesCommand     = cli.Command{
		Name:        "images",
		Usage:       "list images in local storage",
		Description: imagesDescription,
		Flags:       imagesFlags,
		Action:      imagesCmd,
		ArgsUsage:   "",
	}
)

func imagesCmd(c *cli.Context) error {
	config, err := getConfig(c)
	if err != nil {
		return errors.Wrapf(err, "Could not get config")
	}
	store, err := getStore(config)
	if err != nil {
		return err
	}

	quiet := false
	if c.IsSet("quiet") {
		quiet = c.Bool("quiet")
	}
	noheading := false
	if c.IsSet("noheading") {
		noheading = c.Bool("noheading")
	}
	truncate := true
	if c.IsSet("no-trunc") {
		truncate = !c.Bool("no-trunc")
	}
	digests := false
	if c.IsSet("digests") {
		digests = c.Bool("digests")
	}
	outputFormat := genImagesFormat(quiet, truncate, digests)
	if c.IsSet("format") {
		outputFormat = c.String("format")
	}

	name := ""
	if len(c.Args()) == 1 {
		name = c.Args().Get(0)
	} else if len(c.Args()) > 1 {
		return errors.New("'kpod images' requires at most 1 argument")
	}

	var params *libpod.FilterParams
	if c.IsSet("filter") {
		params, err = libpod.ParseFilter(store, c.String("filter"))
		if err != nil {
			return errors.Wrapf(err, "error parsing filter")
		}
	} else {
		params = nil
	}

	imageList, err := libpod.GetImagesMatchingFilter(store, params, name)
	if err != nil {
		return errors.Wrapf(err, "could not get list of images matching filter")
	}

	return outputImages(store, imageList, truncate, digests, quiet, outputFormat, noheading)
}

func genImagesFormat(quiet, truncate, digests bool) (format string) {
	if quiet {
		return formats.IDString
	}
	if truncate {
		format = "table {{ .ID | printf \"%-20.12s\" }} "
	} else {
		format = "table {{ .ID | printf \"%-64s\" }} "
	}
	format += "{{ .Name | printf \"%-56s\" }} "

	if digests {
		format += "{{ .Digest | printf \"%-71s \"}} "
	}

	format += "{{ .CreatedAt | printf \"%-22s\" }} {{.Size}}"
	return
}

func outputImages(store storage.Store, images []storage.Image, truncate, digests, quiet bool, outputFormat string, noheading bool) error {
	imageOutput := []imageOutputParams{}

	lastID := ""
	for _, img := range images {
		if quiet && lastID == img.ID {
			continue // quiet should not show the same ID multiple times
		}
		createdTime := img.Created

		names := []string{""}
		if len(img.Names) > 0 {
			names = img.Names
		}

		info, imageDigest, size, _ := libpod.InfoAndDigestAndSize(store, img)
		if info != nil {
			createdTime = info.Created
		}

		params := imageOutputParams{
			ID:        img.ID,
			Name:      names,
			Digest:    imageDigest,
			CreatedAt: createdTime.Format("Jan 2, 2006 15:04"),
			Size:      libpod.FormattedSize(float64(size)),
		}
		imageOutput = append(imageOutput, params)
	}

	var out formats.Writer

	switch outputFormat {
	case formats.JSONString:
		out = formats.JSONStructArray{Output: toGeneric(imageOutput)}
	default:
		if len(imageOutput) == 0 {
			out = formats.StdoutTemplateArray{}
		} else {
			out = formats.StdoutTemplateArray{Output: toGeneric(imageOutput), Template: outputFormat, Fields: imageOutput[0].headerMap()}
		}
	}

	formats.Writer(out).Out()

	return nil
}

type imageOutputParams struct {
	ID        string        `json:"id"`
	Name      []string      `json:"names"`
	Digest    digest.Digest `json:"digest"`
	CreatedAt string        `json:"created"`
	Size      string        `json:"size"`
}

func toGeneric(params []imageOutputParams) []interface{} {
	genericParams := make([]interface{}, len(params))
	for i, v := range params {
		genericParams[i] = interface{}(v)
	}
	return genericParams
}

func (i *imageOutputParams) headerMap() map[string]string {
	v := reflect.Indirect(reflect.ValueOf(i))
	values := make(map[string]string)

	for i := 0; i < v.NumField(); i++ {
		key := v.Type().Field(i).Name
		value := key
		if value == "ID" || value == "Name" {
			value = "Image" + value
		}
		values[key] = strings.ToUpper(splitCamelCase(value))
	}
	return values
}
