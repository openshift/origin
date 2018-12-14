package buildapihelpers

import (
	"k8s.io/apimachinery/pkg/util/validation"

	buildv1 "github.com/openshift/api/build/v1"
	"github.com/openshift/origin/pkg/api/apihelpers"
)

const (
	// buildPodSuffix is the suffix used to append to a build pod name given a build name
	buildPodSuffix              = "build"
	caConfigMapSuffix           = "ca"
	registryConfConfigMapSuffix = "registry-conf"
)

// GetBuildPodName returns name of the build pod.
func GetBuildPodName(build *buildv1.Build) string {
	return apihelpers.GetPodName(build.Name, buildPodSuffix)
}

// GetBuildCAConfigMapName returns the name of the ConfigMap containing the build's
// certificate authority bundles.
func GetBuildCAConfigMapName(build *buildv1.Build) string {
	return apihelpers.GetConfigMapName(build.Name, caConfigMapSuffix)
}

// GetBuildRegistryConfigMapName returns the name of the ConfigMap containing the build's
// registry configuration.
func GetBuildRegistryConfigMapName(build *buildv1.Build) string {
	return apihelpers.GetConfigMapName(build.Name, registryConfConfigMapSuffix)
}

func StrategyType(strategy buildv1.BuildStrategy) string {
	switch {
	case strategy.DockerStrategy != nil:
		return "Docker"
	case strategy.CustomStrategy != nil:
		return "Custom"
	case strategy.SourceStrategy != nil:
		return "Source"
	case strategy.JenkinsPipelineStrategy != nil:
		return "JenkinsPipeline"
	}
	return ""
}

// LabelValue returns a string to use as a value for the Build
// label in a pod. If the length of the string parameter exceeds
// the maximum label length, the value will be truncated.
func LabelValue(name string) string {
	if len(name) <= validation.DNS1123LabelMaxLength {
		return name
	}
	return name[:validation.DNS1123LabelMaxLength]
}
