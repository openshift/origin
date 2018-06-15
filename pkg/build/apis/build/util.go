package build

import (
	"github.com/openshift/origin/pkg/api/apihelpers"
	"k8s.io/apimachinery/pkg/util/validation"
)

const (
	// BuildPodSuffix is the suffix used to append to a build pod name given a build name
	BuildPodSuffix = "build"
)

// GetBuildPodName returns name of the build pod.
func GetBuildPodName(build *Build) string {
	return apihelpers.GetPodName(build.Name, BuildPodSuffix)
}

func StrategyType(strategy BuildStrategy) string {
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

func SourceType(source BuildSource) string {
	var sourceType string
	if source.Git != nil {
		sourceType = "Git"
	}
	if source.Dockerfile != nil {
		if len(sourceType) != 0 {
			sourceType = sourceType + ","
		}
		sourceType = sourceType + "Dockerfile"
	}
	if source.Binary != nil {
		if len(sourceType) != 0 {
			sourceType = sourceType + ","
		}
		sourceType = sourceType + "Binary"
	}
	return sourceType
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
