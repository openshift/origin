package strategies

import (
	dockerclient "github.com/fsouza/go-dockerclient"
	"github.com/openshift/source-to-image/pkg/api"
	"github.com/openshift/source-to-image/pkg/build"
	"github.com/openshift/source-to-image/pkg/build/strategies/onbuild"
	"github.com/openshift/source-to-image/pkg/build/strategies/sti"
	"github.com/openshift/source-to-image/pkg/docker"
)

// GetStrategy decides what build strategy will be used for the STI build.
func GetStrategy(request *api.Request) (build.Builder, error) {
	image, err := GetBaseImage(request)
	if err != nil {
		return nil, err
	}

	if image.OnBuild {
		return onbuild.New(request)
	}

	return sti.New(request)
}

// GetBaseImage processes the request and performs operations necessary to make
// the Docker image specified as BaseImage available locally.
// It returns information about the base image, containing metadata necessary
// for choosing the right STI build strategy.
func GetBaseImage(request *api.Request) (*docker.PullResult, error) {
	d, err := docker.New(request.DockerConfig)
	result := docker.PullResult{}
	if err != nil {
		return nil, err
	}

	var image *dockerclient.Image
	if request.ForcePull {
		image, err = d.PullImage(request.BaseImage)
	} else {
		image, err = d.CheckAndPull(request.BaseImage)
	}

	if err != nil {
		return nil, err
	}
	result.Image = image
	result.OnBuild = d.IsImageOnBuild(request.BaseImage)
	return &result, nil
}
