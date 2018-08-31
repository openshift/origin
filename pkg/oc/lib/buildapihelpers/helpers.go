package buildapihelpers

import (
	buildv1 "github.com/openshift/api/build/v1"
	buildapi "github.com/openshift/origin/pkg/build/apis/build"
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
		return hasBuildConfigAnnotation(arg.(buildv1.Build), buildapi.BuildConfigAnnotation, labelValue) ||
			hasBuildConfigLabel(arg.(buildv1.Build), buildapi.BuildConfigLabel, labelValue)
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
