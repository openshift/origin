package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/openshift/origin/pkg/generate/app"
	"github.com/openshift/origin/pkg/generate/dockerfile"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

// Generators for ImageRef
// - Name              -> ImageRef
// - ImageRepo + tag   -> ImageRef

// ImageRefGenerator generates ImageRefs
type ImageRefGenerator interface {
	FromName(name string) (*app.ImageRef, error)
	FromNameAndPorts(name string, ports []string) (*app.ImageRef, error)
	FromNameAndResolver(name string, resolver app.Resolver) (*app.ImageRef, error)
	FromStream(repo *imageapi.ImageStream, tag string) (*app.ImageRef, error)
	FromDockerfile(name string, dir string, context string) (*app.ImageRef, error)
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
func (g *imageRefGenerator) FromName(name string) (*app.ImageRef, error) {
	ref, err := imageapi.ParseDockerImageReference(name)
	if err != nil {
		return nil, err
	}
	return &app.ImageRef{
		DockerImageReference: ref,
		AsImageStream:        true,
	}, nil
}

func (g *imageRefGenerator) FromNameAndResolver(name string, resolver app.Resolver) (*app.ImageRef, error) {
	imageRef, err := g.FromName(name)
	if err != nil {
		return nil, err
	}
	imageMatch, err := resolver.Resolve(imageRef.RepoName())
	if multiple, ok := err.(app.ErrMultipleMatches); ok {
		for _, m := range multiple.Matches {
			if m.Image != nil {
				imageMatch = m
				break
			}
		}
	}
	if imageMatch != nil {
		imageRef.Info = imageMatch.Image
	}
	return imageRef, nil
}

func (g *imageRefGenerator) FromNameAndPorts(name string, ports []string) (*app.ImageRef, error) {
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

func (g *imageRefGenerator) FromDockerfile(name string, dir string, context string) (*app.ImageRef, error) {
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
func (g *imageRefGenerator) FromStream(stream *imageapi.ImageStream, tag string) (*app.ImageRef, error) {
	pullSpec := stream.Status.DockerImageRepository
	if len(pullSpec) == 0 {
		// need to know the default OpenShift registry
		return nil, fmt.Errorf("the repository does not resolve to a pullable Docker repository")
	}
	ref, err := imageapi.ParseDockerImageReference(pullSpec)
	if err != nil {
		return nil, err
	}

	if len(tag) == 0 && len(ref.Tag) == 0 {
		ref.Tag = "latest"
	}

	return &app.ImageRef{
		DockerImageReference: ref,
		Stream:               stream,
	}, nil
}
