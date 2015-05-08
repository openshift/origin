package prune

import (
	"fmt"
	"io"

	"github.com/openshift/origin/pkg/dockerregistry/server"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

type summarizingPruner struct {
	delegate ImagePruner
	out      io.Writer

	imageSuccesses []string
	imageFailures  []string
	imageErrors    []error

	/*
		{
			registry1: {
				layer1: {
					requestError: nil,
					layerErrors: [err1, err2],
				},
				...,
			},
			registry2: ...
		}
	*/
	registryResults map[string]registryResult
}

type registryResult struct {
	requestError error
	layerErrors  map[string][]error
}

var _ ImagePruner = &summarizingPruner{}

func NewSummarizingImagePruner(pruner ImagePruner, out io.Writer) ImagePruner {
	return &summarizingPruner{
		delegate:        pruner,
		out:             out,
		registryResults: map[string]registryResult{},
	}
}

func (p *summarizingPruner) Run(baseImagePruneFunc ImagePruneFunc, baseLayerPruneFunc LayerPruneFunc) {
	p.delegate.Run(p.imagePruneFunc(baseImagePruneFunc), p.layerPruneFunc(baseLayerPruneFunc))
	p.summarize()
}

func (p *summarizingPruner) summarize() {
	fmt.Fprintln(p.out, "\nIMAGE PRUNING SUMMARY:")
	fmt.Fprintf(p.out, "# Image prune successes: %d\n", len(p.imageSuccesses))
	fmt.Fprintf(p.out, "# Image prune errors: %d\n", len(p.imageFailures))

	fmt.Fprintln(p.out, "\nLAYER PRUNING SUMMARY:")
	for registry, result := range p.registryResults {
		p.summarizeRegistry(registry, result)
	}
}

func (p *summarizingPruner) summarizeRegistry(registry string, result registryResult) {
	fmt.Fprintf(p.out, "\tRegistry: %s\n", registry)
	fmt.Fprintf(p.out, "\t\tRequest sent successfully: %t\n", result.requestError == nil)
	successes, failures := 0, 0
	for _, errs := range result.layerErrors {
		switch len(errs) {
		case 0:
			successes++
		default:
			failures++
		}
	}
	fmt.Fprintf(p.out, "\t\t# Layer prune successes: %d\n", successes)
	fmt.Fprintf(p.out, "\t\t# Layer prune errors: %d\n", failures)
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
	return func(registryURL string, req server.DeleteLayersRequest) (error, map[string][]error) {
		requestError, layerErrors := base(registryURL, req)
		p.registryResults[registryURL] = registryResult{
			requestError: requestError,
			layerErrors:  layerErrors,
		}
		return requestError, layerErrors
	}
}
