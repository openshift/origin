package buildapihelpers

import (
	coreinternalapi "k8s.io/kubernetes/pkg/apis/core"

	buildinternalapi "github.com/openshift/origin/pkg/build/apis/build"
)

// BuildToPodLogOptions builds a PodLogOptions object out of a BuildLogOptions.
// Currently BuildLogOptions.Container and BuildLogOptions.Previous aren't used
// so they won't be copied to PodLogOptions.
func BuildToPodLogOptions(opts *buildinternalapi.BuildLogOptions) *coreinternalapi.PodLogOptions {
	return &coreinternalapi.PodLogOptions{
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
func FindTriggerPolicy(triggerType buildinternalapi.BuildTriggerType, config *buildinternalapi.BuildConfig) (buildTriggers []buildinternalapi.BuildTriggerPolicy) {
	for _, specTrigger := range config.Spec.Triggers {
		if specTrigger.Type == triggerType {
			buildTriggers = append(buildTriggers, specTrigger)
		}
	}
	return buildTriggers
}

func HasTriggerType(triggerType buildinternalapi.BuildTriggerType, bc *buildinternalapi.BuildConfig) bool {
	matches := FindTriggerPolicy(triggerType, bc)
	return len(matches) > 0
}

// GetInputReference returns the From ObjectReference associated with the
// BuildStrategy.
func GetInputReference(strategy buildinternalapi.BuildStrategy) *coreinternalapi.ObjectReference {
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
