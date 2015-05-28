package util

import (
	"strings"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"

	buildapi "github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/util/podname"
)

// BuildPodSuffix is the suffix used to append to a build pod name given a build name
const BuildPodSuffix = "build"

// GetBuildPodName returns name of the build pod.
func GetBuildPodName(build *buildapi.Build) string {
	return podname.GetName(build.Name, BuildPodSuffix)
}

// GetBuildName returns name of the build pod.
func GetBuildName(pod *kapi.Pod) string {
	if pod.Annotations == nil {
		return ""
	}
	return pod.Annotations[buildapi.BuildAnnotation]
}

// GetImageStreamForStrategy returns the ImageStream[Tag/Image] ObjectReference associated
// with the BuildStrategy.
func GetImageStreamForStrategy(strategy buildapi.BuildStrategy) *kapi.ObjectReference {
	switch strategy.Type {
	case buildapi.SourceBuildStrategyType:
		return strategy.SourceStrategy.From
	case buildapi.DockerBuildStrategyType:
		return strategy.DockerStrategy.From
	case buildapi.CustomBuildStrategyType:
		return strategy.CustomStrategy.From
	default:
		return nil
	}
}

// NameFromImageStream returns a concatenated name representing an ImageStream[Tag/Image]
// reference.  If the reference does not contain a Namespace, the namespace parameter
// is used instead.
func NameFromImageStream(namespace string, ref *kapi.ObjectReference, tag string) string {
	var ret string
	if ref.Namespace == "" {
		ret = namespace
	} else {
		ret = ref.Namespace
	}
	ret = ret + "/" + ref.Name
	if tag != "" && strings.Index(ref.Name, ":") == -1 && strings.Index(ref.Name, "@") == -1 {
		ret = ret + ":" + tag
	}
	return ret
}

func IsBuildComplete(build *buildapi.Build) bool {
	if build.Status != buildapi.BuildStatusRunning && build.Status != buildapi.BuildStatusPending && build.Status != buildapi.BuildStatusNew {
		return true
	}
	return false
}
