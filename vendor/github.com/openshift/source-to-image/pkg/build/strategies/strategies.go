package strategies

import (
	"github.com/openshift/source-to-image/pkg/api"
	"github.com/openshift/source-to-image/pkg/build"
	"github.com/openshift/source-to-image/pkg/build/strategies/onbuild"
	"github.com/openshift/source-to-image/pkg/build/strategies/sti"
	"github.com/openshift/source-to-image/pkg/docker"
	utilstatus "github.com/openshift/source-to-image/pkg/util/status"
)

// GetStrategy decides what build strategy will be used for the STI build.
// TODO: deprecated, use Strategy() instead
func GetStrategy(config *api.Config) (build.Builder, api.BuildInfo, error) {
	return Strategy(config, build.Overrides{})
}

// Strategy creates the appropriate build strategy for the provided config, using
// the overrides provided. Not all strategies support all overrides.
func Strategy(config *api.Config, overrides build.Overrides) (build.Builder, api.BuildInfo, error) {
	var builder build.Builder
	var buildInfo api.BuildInfo

	image, err := docker.GetBuilderImage(config)
	if err != nil {
		buildInfo.FailureReason = utilstatus.NewFailureReason(utilstatus.ReasonPullBuilderImageFailed, utilstatus.ReasonMessagePullBuilderImageFailed)
		return nil, buildInfo, err
	}
	config.HasOnBuild = image.OnBuild

	// if we're blocking onbuild, just do a normal s2i build flow
	// which won't do a docker build and invoke the onbuild commands
	if image.OnBuild && !config.BlockOnBuild {
		builder, err = onbuild.New(config, overrides)
		if err != nil {
			buildInfo.FailureReason = utilstatus.NewFailureReason(utilstatus.ReasonGenericS2IBuildFailed, utilstatus.ReasonMessageGenericS2iBuildFailed)
			return nil, buildInfo, err
		}
		return builder, buildInfo, nil
	}

	builder, err = sti.New(config, overrides)
	if err != nil {
		buildInfo.FailureReason = utilstatus.NewFailureReason(utilstatus.ReasonGenericS2IBuildFailed, utilstatus.ReasonMessageGenericS2iBuildFailed)
		return nil, buildInfo, err
	}
	return builder, buildInfo, err
}
