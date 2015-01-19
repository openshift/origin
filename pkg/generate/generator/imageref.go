package generator

import (
	"fmt"

	"github.com/openshift/origin/pkg/generate/app"
	image "github.com/openshift/origin/pkg/image/api"
)

// Generators for ImageRef
// - Name              -> ImageRef
// - ImageRepo + tag   -> ImageRef

// ImageRefGenerator generates ImageRefs
type ImageRefGenerator interface {
	FromName(name string) (*app.ImageRef, error)
	FromRepository(repo *image.ImageRepository, tag string) (*app.ImageRef, error)
}

type imageRefGenerator struct{}

// NewImageRefGenerator creates a new ImageRefGenerator
func NewImageRefGenerator() ImageRefGenerator {
	return &imageRefGenerator{}
}

// FromName generates an ImageRef from a given name
func (g *imageRefGenerator) FromName(name string) (*app.ImageRef, error) {
	registry, namespace, name, tag, err := image.SplitDockerPullSpec(name)
	if err != nil {
		return nil, err
	}
	return &app.ImageRef{
		Registry:  registry,
		Namespace: namespace,
		Name:      name,
		Tag:       tag,
	}, nil
}

// FromRepository generates an ImageRef from an OpenShift ImageRepository
func (g *imageRefGenerator) FromRepository(repo *image.ImageRepository, tag string) (*app.ImageRef, error) {
	pullSpec := repo.Status.DockerImageRepository
	if len(pullSpec) == 0 {
		// need to know the default OpenShift registry
		return nil, fmt.Errorf("the repository does not resolve to a pullable Docker repository")
	}
	registry, namespace, name, repoTag, err := image.SplitDockerPullSpec(pullSpec)
	if err != nil {
		return nil, err
	}

	if len(tag) == 0 {
		if len(repoTag) != 0 {
			tag = repoTag
		} else {
			tag = "latest"
		}
	}

	return &app.ImageRef{
		Registry:  registry,
		Namespace: namespace,
		Name:      name,
		Tag:       tag,

		Repository: repo,
	}, nil
}
