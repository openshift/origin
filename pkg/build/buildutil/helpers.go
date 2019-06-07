package buildutil

import (
	buildv1 "github.com/openshift/api/build/v1"
)

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
