package buildapihelpers

import (
	buildv1 "github.com/openshift/api/build/v1"
	corev1 "k8s.io/api/core/v1"
)

// BuildToPodLogOptions builds a PodLogOptions object out of a BuildLogOptions.
// Currently BuildLogOptions.Container and BuildLogOptions.Previous aren't used
// so they won't be copied to PodLogOptions.
func BuildToPodLogOptions(opts *buildv1.BuildLogOptions) *corev1.PodLogOptions {
	return &corev1.PodLogOptions{
		Follow:       opts.Follow,
		SinceSeconds: opts.SinceSeconds,
		SinceTime:    opts.SinceTime,
		Timestamps:   opts.Timestamps,
		TailLines:    opts.TailLines,
		LimitBytes:   opts.LimitBytes,
	}
}

// FindTriggerPolicy retrieves the BuildTrigger(s) of a given type from a build configuration.
// Returns nil if no matches are found.
func FindTriggerPolicy(triggerType buildv1.BuildTriggerType, config *buildv1.BuildConfig) (buildTriggers []buildv1.BuildTriggerPolicy) {
	for _, specTrigger := range config.Spec.Triggers {
		if specTrigger.Type == triggerType {
			buildTriggers = append(buildTriggers, specTrigger)
		}
	}
	return buildTriggers
}

func HasTriggerType(triggerType buildv1.BuildTriggerType, bc *buildv1.BuildConfig) bool {
	matches := FindTriggerPolicy(triggerType, bc)
	return len(matches) > 0
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
