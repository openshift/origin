package image

import (
	"fmt"
	"io"
	"strings"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubecfg"
	"github.com/openshift/origin/pkg/image/api"
)

var imageColumns = []string{"ID", "Docker Ref"}
var imageRepositoryColumns = []string{"ID", "Docker Repo", "Tags"}

// RegisterPrintHandlers registers HumanReadablePrinter handlers for image and image repository resources.
func RegisterPrintHandlers(printer *kubecfg.HumanReadablePrinter) {
	printer.Handler(imageColumns, printImage)
	printer.Handler(imageColumns, printImageList)
	printer.Handler(imageRepositoryColumns, printImageRepository)
	printer.Handler(imageRepositoryColumns, printImageRepositoryList)
}

func printImage(image *api.Image, w io.Writer) error {
	_, err := fmt.Fprintf(w, "%s\t%s\n", image.Name, image.DockerImageReference)
	return err
}

func printImageList(images *api.ImageList, w io.Writer) error {
	for _, image := range images.Items {
		if err := printImage(&image, w); err != nil {
			return err
		}
	}
	return nil
}

func printImageRepository(repo *api.ImageRepository, w io.Writer) error {
	tags := ""
	if len(repo.Tags) > 0 {
		var t []string
		for tag := range repo.Tags {
			t = append(t, tag)
		}
		tags = strings.Join(t, ",")
	}
	_, err := fmt.Fprintf(w, "%s\t%s\t%s\n", repo.Name, repo.DockerImageRepository, tags)
	return err
}

func printImageRepositoryList(repos *api.ImageRepositoryList, w io.Writer) error {
	for _, repo := range repos.Items {
		if err := printImageRepository(&repo, w); err != nil {
			return err
		}
	}
	return nil
}
