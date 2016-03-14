package util

import (
	"fmt"
	"strconv"
	"strings"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/labels"

	buildapi "github.com/openshift/origin/pkg/build/api"
)

const (
	// NoBuildLogsMessage reports that no build logs are available
	NoBuildLogsMessage = "No logs are available."
)

// GetBuildPodName returns name of the build pod.
// TODO: remove in favor of the one in the api package
func GetBuildPodName(build *buildapi.Build) string {
	return buildapi.GetBuildPodName(build)
}

// GetBuildName returns name of the build pod.
func GetBuildName(pod *kapi.Pod) string {
	if pod == nil {
		return ""
	}
	return pod.Annotations[buildapi.BuildAnnotation]
}

// GetInputReference returns the From ObjectReference associated with the
// BuildStrategy.
func GetInputReference(strategy buildapi.BuildStrategy) *kapi.ObjectReference {
	switch {
	case strategy.SourceStrategy != nil:
		return &strategy.SourceStrategy.From
	case strategy.DockerStrategy != nil:
		return strategy.DockerStrategy.From
	case strategy.CustomStrategy != nil:
		return &strategy.CustomStrategy.From
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

// IsBuildComplete returns whether the provided build is complete or not
func IsBuildComplete(build *buildapi.Build) bool {
	return build.Status.Phase != buildapi.BuildPhaseRunning && build.Status.Phase != buildapi.BuildPhasePending && build.Status.Phase != buildapi.BuildPhaseNew
}

// IsPaused returns true if the provided BuildConfig is paused and cannot be used to create a new Build
func IsPaused(bc *buildapi.BuildConfig) bool {
	return strings.ToLower(bc.Annotations[buildapi.BuildConfigPausedAnnotation]) == "true"
}

// BuildNameForConfigVersion returns the name of the version-th build
// for the config that has the provided name.
func BuildNameForConfigVersion(name string, version int) string {
	return fmt.Sprintf("%s-%d", name, version)
}

// BuildConfigSelector returns a label Selector which can be used to find all
// builds for a BuildConfig.
func BuildConfigSelector(name string) labels.Selector {
	return labels.Set{buildapi.BuildConfigLabel: name}.AsSelector()
}

// BuildConfigSelectorDeprecated returns a label Selector which can be used to find
// all builds for a BuildConfig that use the deprecated labels.
func BuildConfigSelectorDeprecated(name string) labels.Selector {
	return labels.Set{buildapi.BuildConfigLabelDeprecated: name}.AsSelector()
}

// ConfigNameForBuild returns the name of the build config from a
// build name.
func ConfigNameForBuild(build *buildapi.Build) string {
	if build == nil {
		return ""
	}
	if _, exists := build.Labels[buildapi.BuildConfigLabel]; exists {
		return build.Labels[buildapi.BuildConfigLabel]
	}
	return build.Labels[buildapi.BuildConfigLabelDeprecated]
}

// VersionForBuild returns the version from the provided build name.
// If no version can be found, 0 is returned to indicate no version.
func VersionForBuild(build *buildapi.Build) int {
	if build == nil {
		return 0
	}
	versionString := build.Annotations[buildapi.BuildNumberAnnotation]
	version, err := strconv.Atoi(versionString)
	if err != nil {
		return 0
	}
	return version
}
