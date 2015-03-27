package util

import (
	buildapi "github.com/openshift/origin/pkg/build/api"
)

// GetBuildPodName returns name of the build pod.
func GetBuildPodName(build *buildapi.Build) string {
	return build.Name
}
