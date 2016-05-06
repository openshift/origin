package webhook

import (
	"crypto/hmac"
	"errors"
	"strings"

	"github.com/openshift/origin/pkg/build/api"
)

const (
	refPrefix        = "refs/heads/"
	DefaultConfigRef = "master"
)

var (
	ErrSecretMismatch = errors.New("the provided secret does not match")
	ErrHookNotEnabled = errors.New("the specified hook is not enabled")
)

// GitRefMatches determines if the ref from a webhook event matches a build
// configuration
func GitRefMatches(eventRef, configRef string, buildSource *api.BuildSource) bool {
	if buildSource.Git != nil && len(buildSource.Git.Ref) != 0 {
		configRef = buildSource.Git.Ref
	}

	eventRef = strings.TrimPrefix(eventRef, refPrefix)
	configRef = strings.TrimPrefix(configRef, refPrefix)
	return configRef == eventRef
}

// FindTriggerPolicy retrieves the BuildTrigger of a given type from a build
// configuration
func FindTriggerPolicy(triggerType api.BuildTriggerType, config *api.BuildConfig) (buildTriggers []api.BuildTriggerPolicy, err error) {
	err = ErrHookNotEnabled
	for _, specTrigger := range config.Spec.Triggers {
		if specTrigger.Type == triggerType {
			buildTriggers = append(buildTriggers, specTrigger)
			err = nil
		}
	}
	return buildTriggers, err
}

// ValidateWebHookSecret validates the provided secret against all currently
// defined webhook secrets and if it is valid, returns its information.
func ValidateWebHookSecret(webHookTriggers []api.BuildTriggerPolicy, secret string) (*api.WebHookTrigger, error) {
	for _, trigger := range webHookTriggers {
		if trigger.Type == api.GenericWebHookBuildTriggerType {
			if !hmac.Equal([]byte(trigger.GenericWebHook.Secret), []byte(secret)) {
				continue
			}
			return trigger.GenericWebHook, nil
		}
		if trigger.Type == api.GitHubWebHookBuildTriggerType {
			if !hmac.Equal([]byte(trigger.GitHubWebHook.Secret), []byte(secret)) {
				continue
			}
			return trigger.GitHubWebHook, nil
		}
	}
	return nil, ErrSecretMismatch
}
