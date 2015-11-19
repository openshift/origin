package strategies

import (
	"github.com/openshift/source-to-image/pkg/api"
	"github.com/openshift/source-to-image/pkg/build"
	"github.com/openshift/source-to-image/pkg/build/strategies/onbuild"
	"github.com/openshift/source-to-image/pkg/build/strategies/sti"
	"github.com/openshift/source-to-image/pkg/docker"
)

// GetStrategy decides what build strategy will be used for the STI build.
// TODO: deprecated, use Strategy() instead
func GetStrategy(config *api.Config) (build.Builder, error) {
	return Strategy(config, build.Overrides{})
}

// Strategy creates the appropriate build strategy for the provided config, using
// the overrides provided. Not all strategies support all overrides.
func Strategy(config *api.Config, overrides build.Overrides) (build.Builder, error) {
	image, err := docker.GetBuilderImage(config)
	if err != nil {
		return nil, err
	}
	if image.OnBuild {
		return onbuild.New(config, overrides)
	}
	return sti.New(config, overrides)
}
