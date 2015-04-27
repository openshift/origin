package prune

import (
	"fmt"
	"io"

	"github.com/openshift/origin/pkg/cmd/dockerregistry"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

type summarizingPruner struct {
	delegate ImagePruner
	out      io.Writer

	imageSuccesses []string
	imageFailures  []string
	imageErrors    []error

	layerSuccesses []string
	layerFailures  []string
	layerErrors    []error
}

var _ ImagePruner = &summarizingPruner{}

func NewSummarizingImagePruner(pruner ImagePruner, out io.Writer) ImagePruner {
	return &summarizingPruner{
		delegate: pruner,
		out:      out,
	}
}

func (p *summarizingPruner) Run(baseImagePruneFunc ImagePruneFunc, baseLayerPruneFunc LayerPruneFunc) {
	p.delegate.Run(p.imagePruneFunc(baseImagePruneFunc), p.layerPruneFunc(baseLayerPruneFunc))
	p.summarize()
}

func (p *summarizingPruner) summarize() {
	fmt.Fprintln(p.out, "IMAGE PRUNING SUMMARY:")
	fmt.Fprintf(p.out, "# Image prune successes: %d\n", len(p.imageSuccesses))
	fmt.Fprintf(p.out, "# Image prune errors: %d\n", len(p.imageFailures))
	fmt.Fprintln(p.out, "LAYER PRUNING SUMMARY:")
	fmt.Fprintf(p.out, "# Layer prune successes: %d\n", len(p.layerSuccesses))
	fmt.Fprintf(p.out, "# Layer prune errors: %d\n", len(p.layerFailures))
}

func (p *summarizingPruner) imagePruneFunc(base ImagePruneFunc) ImagePruneFunc {
	return func(image *imageapi.Image, streams []*imageapi.ImageStream) []error {
		errs := base(image, streams)
		switch len(errs) {
		case 0:
			p.imageSuccesses = append(p.imageSuccesses, image.Name)
		default:
			p.imageFailures = append(p.imageFailures, image.Name)
			p.imageErrors = append(p.imageErrors, errs...)
		}
		return errs
	}
}

func (p *summarizingPruner) layerPruneFunc(base LayerPruneFunc) LayerPruneFunc {
	return func(registryURL string, req dockerregistry.DeleteLayersRequest) []error {
		errs := base(registryURL, req)
		return errs
	}
}
