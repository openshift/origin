package app

import (
	"fmt"

	image "github.com/openshift/origin/pkg/image/api"
)

func ImageFromName(name string, tag string) (*ImageRef, error) {
	ref, err := image.ParseDockerImageReference(name)
	if err != nil {
		return nil, err
	}

	if len(tag) == 0 {
		if len(ref.Tag) != 0 {
			tag = ref.Tag
		} else {
			tag = "latest"
		}
	}

	return &ImageRef{
		DockerImageReference: ref,
	}, nil
}

func ImageFromRepository(stream *image.ImageStream, tag string) (*ImageRef, error) {
	pullSpec := stream.Status.DockerImageRepository
	if len(pullSpec) == 0 {
		// need to know the default OpenShift registry
		return nil, fmt.Errorf("the stream does not resolve to a pullable Docker repository")
	}

	ref, err := ImageFromName(pullSpec, tag)
	if err != nil {
		return nil, err
	}

	ref.Stream = stream
	return ref, nil
}
