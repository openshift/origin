package buildapihelpers

import (
	"github.com/openshift/origin/pkg/api/apihelpers"
	buildinternalapi "github.com/openshift/origin/pkg/build/apis/build"
	"k8s.io/apimachinery/pkg/util/validation"
)

const (
	// buildPodSuffix is the suffix used to append to a build pod name given a build name
	buildPodSuffix = "build"
)

// GetBuildPodName returns name of the build pod.
func GetBuildPodName(build *buildinternalapi.Build) string {
	return apihelpers.GetPodName(build.Name, buildPodSuffix)
}

func StrategyType(strategy buildinternalapi.BuildStrategy) string {
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
