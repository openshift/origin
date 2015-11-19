package api

import (
	kapi "k8s.io/kubernetes/pkg/api"
)

// BuildToPodLogOptions builds a PodLogOptions object out of a BuildLogOptions.
// Currently BuildLogOptions.Container and BuildLogOptions.Previous aren't used
// so they won't be copied to PodLogOptions.
func BuildToPodLogOptions(opts *BuildLogOptions) *kapi.PodLogOptions {
	return &kapi.PodLogOptions{
		Follow:       opts.Follow,
		SinceSeconds: opts.SinceSeconds,
		SinceTime:    opts.SinceTime,
		Timestamps:   opts.Timestamps,
		TailLines:    opts.TailLines,
		LimitBytes:   opts.LimitBytes,
	}
}

// PredicateFunc is testing an argument and decides does it meet some criteria or not.
// It can be used for filtering elements based on some conditions.
type PredicateFunc func(interface{}) bool

// FilterBuilds returns array of builds that satisfies predicate function.
func FilterBuilds(builds []Build, predicate PredicateFunc) []Build {
	if len(builds) == 0 {
		return builds
	}

	result := make([]Build, 0)
	for _, build := range builds {
		if predicate(build) {
			result = append(result, build)
		}
	}

	return result
}

// ByBuildConfigLabelPredicate matches all builds that have build config label with specified value.
func ByBuildConfigLabelPredicate(labelValue string) PredicateFunc {
	return func(arg interface{}) bool {
		return (hasBuildConfigLabel(arg.(Build), BuildConfigLabel, labelValue) ||
			hasBuildConfigLabel(arg.(Build), BuildConfigLabelDeprecated, labelValue))
	}
}

func hasBuildConfigLabel(build Build, labelName, labelValue string) bool {
	value, ok := build.Labels[labelName]
	return ok && value == labelValue
}
