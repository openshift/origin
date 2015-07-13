package webhook

import (
	"fmt"
	"strings"

	"github.com/openshift/origin/pkg/build/api"
)

var (
	ErrSecretMismatch = fmt.Errorf("the provided secret does not match")
	ErrHookNotEnabled = fmt.Errorf("the specified hook is not enabled")
)

// GitRefMatches determines if the ref from a webhook event matches a build configuration
func GitRefMatches(eventRef, configRef string) bool {
	const RefPrefix = "refs/heads/"
	eventRef = strings.TrimPrefix(eventRef, RefPrefix)
	configRef = strings.TrimPrefix(configRef, RefPrefix)
	if configRef == "" {
		configRef = "master"
	}
	return configRef == eventRef
}

// FindTriggerPolicy retrieves the BuildTrigger of a given type from a build configuration
func FindTriggerPolicy(triggerType api.BuildTriggerType, config *api.BuildConfig) (*api.BuildTriggerPolicy, bool) {
	for _, p := range config.Spec.Triggers {
		if p.Type == triggerType {
			return &p, true
		}
	}
	return nil, false
}
