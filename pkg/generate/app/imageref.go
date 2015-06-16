package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/openshift/origin/pkg/generate/dockerfile"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

// ImageRefGenerator is an interface for generating ImageRefs
//
// Generators for ImageRef
// - Name              -> ImageRef
// - ImageRepo + tag   -> ImageRef
type ImageRefGenerator interface {
	FromName(name string) (*ImageRef, error)
	FromNameAndPorts(name string, ports []string) (*ImageRef, error)
	FromStream(repo *imageapi.ImageStream, tag string) (*ImageRef, error)
	FromDockerfile(name string, dir string, context string) (*ImageRef, error)
}

type imageRefGenerator struct {
	dockerParser dockerfile.Parser
}

// NewImageRefGenerator creates a new ImageRefGenerator
func NewImageRefGenerator() ImageRefGenerator {
	return &imageRefGenerator{
		dockerParser: dockerfile.NewParser(),
	}
}

// FromName generates an ImageRef from a given name
func (g *imageRefGenerator) FromName(name string) (*ImageRef, error) {
	ref, err := imageapi.ParseDockerImageReference(name)
	if err != nil {
		return nil, err
	}
	return &ImageRef{
		DockerImageReference: ref,
	}, nil
}

// FromNameAndPorts generates an ImageRef from a given name and ports
func (g *imageRefGenerator) FromNameAndPorts(name string, ports []string) (*ImageRef, error) {
	present := struct{}{}
	imageRef, err := g.FromName(name)
	if err != nil {
		return nil, err
	}
	exposedPorts := map[string]struct{}{}

	for _, p := range ports {
		exposedPorts[p] = present
	}

	imageRef.Info = &imageapi.DockerImage{
		Config: imageapi.DockerConfig{
			ExposedPorts: exposedPorts,
		},
	}
	return imageRef, nil
}

// FromDockerfile generates an ImageRef from a given name, directory, and context path.
// The directory and context path will be joined and the resulting path should be a
// Dockerfile from where the image's ports will be extracted.
func (g *imageRefGenerator) FromDockerfile(name string, dir string, context string) (*ImageRef, error) {
	// Look for Dockerfile in repository
	file, err := os.Open(filepath.Join(dir, context, "Dockerfile"))
	if err != nil {
		return nil, err
	}

	dockerFile, err := g.dockerParser.Parse(file)
	if err != nil {
		return nil, err
	}

	expose, ok := dockerFile.GetDirective("EXPOSE")
	if !ok {
		return nil, err
	}
	ports := []string{}
	for _, e := range expose {
		ps := strings.Split(e, " ")
		ports = append(ports, ps...)
	}
	return g.FromNameAndPorts(name, ports)
}

// FromStream generates an ImageRef from an OpenShift ImageStream
func (g *imageRefGenerator) FromStream(stream *imageapi.ImageStream, tag string) (*ImageRef, error) {
	pullSpec := stream.Status.DockerImageRepository
	if len(pullSpec) == 0 {
		// need to know the default OpenShift registry
		return nil, fmt.Errorf("the repository does not resolve to a pullable Docker repository")
	}
	ref, err := imageapi.ParseDockerImageReference(pullSpec)
	if err != nil {
		return nil, err
	}

	switch {
	case len(tag) > 0:
		ref.Tag = tag
	case len(tag) == 0 && len(ref.Tag) == 0:
		ref.Tag = imageapi.DefaultImageTag
	}

	return &ImageRef{
		DockerImageReference: ref,
		Stream:               stream,
	}, nil
}
