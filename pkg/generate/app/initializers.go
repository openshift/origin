package app

import (
	"fmt"

	image "github.com/openshift/origin/pkg/image/api"
)

func ImageFromName(name string, tag string) (*ImageRef, error) {
	registry, namespace, name, repoTag, err := image.SplitDockerPullSpec(name)
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

	return &ImageRef{
		Registry:  registry,
		Namespace: namespace,
		Name:      name,
		Tag:       tag,
	}, nil
}

func ImageFromRepository(repo *image.ImageRepository, tag string) (*ImageRef, error) {
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

	return &ImageRef{
		Registry:  registry,
		Namespace: namespace,
		Name:      name,
		Tag:       tag,

		Repository: repo,
	}, nil
}
