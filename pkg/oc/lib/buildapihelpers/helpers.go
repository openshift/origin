package buildapihelpers

import (
	buildinternalapi "github.com/openshift/origin/pkg/build/apis/build"
)

// PredicateFunc is testing an argument and decides does it meet some criteria or not.
// It can be used for filtering elements based on some conditions.
type PredicateFunc func(interface{}) bool

// FilterBuilds returns array of builds that satisfies predicate function.
func FilterBuilds(builds []buildinternalapi.Build, predicate PredicateFunc) []buildinternalapi.Build {
	if len(builds) == 0 {
		return builds
	}

	result := make([]buildinternalapi.Build, 0)
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
		return hasBuildConfigAnnotation(arg.(buildinternalapi.Build), buildinternalapi.BuildConfigAnnotation, labelValue) ||
			hasBuildConfigLabel(arg.(buildinternalapi.Build), buildinternalapi.BuildConfigLabel, labelValue) ||
			hasBuildConfigLabel(arg.(buildinternalapi.Build), buildinternalapi.BuildConfigLabelDeprecated, labelValue)
	}
}

func hasBuildConfigLabel(build buildinternalapi.Build, labelName, labelValue string) bool {
	value, ok := build.Labels[labelName]
	return ok && value == labelValue
}

func hasBuildConfigAnnotation(build buildinternalapi.Build, annotationName, annotationValue string) bool {
	if build.Annotations == nil {
		return false
	}
	value, ok := build.Annotations[annotationName]
	return ok && value == annotationValue
}
