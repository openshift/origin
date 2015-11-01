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
