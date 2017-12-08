package build

import (
	kapi "k8s.io/kubernetes/pkg/apis/core"
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

// ByBuildConfigPredicate matches all builds that have build config annotation or label with specified value.
func ByBuildConfigPredicate(labelValue string) PredicateFunc {
	return func(arg interface{}) bool {
		return (hasBuildConfigAnnotation(arg.(Build), BuildConfigAnnotation, labelValue) ||
			hasBuildConfigLabel(arg.(Build), BuildConfigLabel, labelValue) ||
			hasBuildConfigLabel(arg.(Build), BuildConfigLabelDeprecated, labelValue))
	}
}

func hasBuildConfigLabel(build Build, labelName, labelValue string) bool {
	value, ok := build.Labels[labelName]
	return ok && value == labelValue
}

func hasBuildConfigAnnotation(build Build, annotationName, annotationValue string) bool {
	if build.Annotations == nil {
		return false
	}
	value, ok := build.Annotations[annotationName]
	return ok && value == annotationValue
}

// FindTriggerPolicy retrieves the BuildTrigger(s) of a given type from a build configuration.
// Returns nil if no matches are found.
func FindTriggerPolicy(triggerType BuildTriggerType, config *BuildConfig) (buildTriggers []BuildTriggerPolicy) {
	for _, specTrigger := range config.Spec.Triggers {
		if specTrigger.Type == triggerType {
			buildTriggers = append(buildTriggers, specTrigger)
		}
	}
	return buildTriggers
}

func HasTriggerType(triggerType BuildTriggerType, bc *BuildConfig) bool {
	matches := FindTriggerPolicy(triggerType, bc)
	return len(matches) > 0
}

// GetInputReference returns the From ObjectReference associated with the
// BuildStrategy.
func GetInputReference(strategy BuildStrategy) *kapi.ObjectReference {
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
