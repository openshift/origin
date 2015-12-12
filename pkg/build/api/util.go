package api

import "github.com/openshift/origin/pkg/util/namer"

const (
	// BuildPodSuffix is the suffix used to append to a build pod name given a build name
	BuildPodSuffix = "build"
)

// GetBuildPodName returns name of the build pod.
func GetBuildPodName(build *Build) string {
	return namer.GetPodName(build.Name, BuildPodSuffix)
}

func StrategyType(strategy BuildStrategy) string {
	switch {
	case strategy.DockerStrategy != nil:
		return "Docker"
	case strategy.CustomStrategy != nil:
		return "Custom"
	case strategy.SourceStrategy != nil:
		return "Source"
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
