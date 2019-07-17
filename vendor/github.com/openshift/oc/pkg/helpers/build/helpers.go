package build

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"

	buildv1 "github.com/openshift/api/build/v1"
)

// PredicateFunc is testing an argument and decides does it meet some criteria or not.
type PredicateFunc func(interface{}) bool

// FilterBuilds returns array of builds that satisfies predicate function.
func FilterBuilds(builds []buildv1.Build, predicate PredicateFunc) []buildv1.Build {
	if len(builds) == 0 {
		return builds
	}

	result := make([]buildv1.Build, 0)
	for _, build := range builds {
		if predicate(build) {
			result = append(result, build)
		}
	}

	return result
}

// ByBuildConfigPredicate matches all builds that have build config annotation or label with specified value.
func ByBuildConfigPredicate(labelValue string) PredicateFunc {
	return func(arg interface{}) bool {
		return hasBuildConfigAnnotation(arg.(buildv1.Build), buildv1.BuildConfigAnnotation, labelValue) ||
			hasBuildConfigLabel(arg.(buildv1.Build), buildv1.BuildConfigLabel, labelValue)
	}
}

func hasBuildConfigLabel(build buildv1.Build, labelName, labelValue string) bool {
	value, ok := build.Labels[labelName]
	return ok && value == labelValue
}

func hasBuildConfigAnnotation(build buildv1.Build, annotationName, annotationValue string) bool {
	if build.Annotations == nil {
		return false
	}
	value, ok := build.Annotations[annotationName]
	return ok && value == annotationValue
}

// BuildNameForConfigVersion returns the name of the version-th build
// for the config that has the provided name.
func BuildNameForConfigVersion(name string, version int) string {
	return fmt.Sprintf("%s-%d", name, version)
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

// IsBuildComplete returns whether the provided build is complete or not
func IsBuildComplete(build *buildv1.Build) bool {
	return IsTerminalPhase(build.Status.Phase)
}

// IsTerminalPhase returns true if the provided phase is terminal
func IsTerminalPhase(phase buildv1.BuildPhase) bool {
	switch phase {
	case buildv1.BuildPhaseNew,
		buildv1.BuildPhasePending,
		buildv1.BuildPhaseRunning:
		return false
	}
	return true
}

// GetInputReference returns the From ObjectReference associated with the
// BuildStrategy.
func GetInputReference(strategy buildv1.BuildStrategy) *corev1.ObjectReference {
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
