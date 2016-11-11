package webhook

import (
	"crypto/hmac"
	"errors"
	"net/http"
	"strings"

	kapi "k8s.io/kubernetes/pkg/api"
	kerrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/unversioned"

	buildapi "github.com/openshift/origin/pkg/build/api"
)

const (
	refPrefix        = "refs/heads/"
	DefaultConfigRef = "master"
)

var (
	ErrSecretMismatch  = errors.New("the provided secret does not match")
	ErrHookNotEnabled  = errors.New("the specified hook is not enabled")
	MethodNotSupported = errors.New("unsupported HTTP method")
)

// Plugin for Webhook verification is dependent on the sending side, it can be
// eg. github, bitbucket or else, so there must be a separate Plugin
// instance for each webhook provider.
type Plugin interface {
	// Method extracts build information and returns:
	// - newly created build object or nil if default is to be created
	// - information whether to trigger the build itself
	// - eventual error.
	Extract(buildCfg *buildapi.BuildConfig, secret, path string, req *http.Request) (*buildapi.SourceRevision, []kapi.EnvVar, bool, error)
}

// GitRefMatches determines if the ref from a webhook event matches a build
// configuration
func GitRefMatches(eventRef, configRef string, buildSource *buildapi.BuildSource) bool {
	if buildSource.Git != nil && len(buildSource.Git.Ref) != 0 {
		configRef = buildSource.Git.Ref
	}

	eventRef = strings.TrimPrefix(eventRef, refPrefix)
	configRef = strings.TrimPrefix(configRef, refPrefix)
	return configRef == eventRef
}

// FindTriggerPolicy retrieves the BuildTrigger of a given type from a build
// configuration
func FindTriggerPolicy(triggerType buildapi.BuildTriggerType, config *buildapi.BuildConfig) (buildTriggers []buildapi.BuildTriggerPolicy, err error) {
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
func ValidateWebHookSecret(webHookTriggers []buildapi.BuildTriggerPolicy, secret string) (*buildapi.WebHookTrigger, error) {
	for _, trigger := range webHookTriggers {
		if trigger.Type == buildapi.GenericWebHookBuildTriggerType {
			if !hmac.Equal([]byte(trigger.GenericWebHook.Secret), []byte(secret)) {
				continue
			}
			return trigger.GenericWebHook, nil
		}
		if trigger.Type == buildapi.GitHubWebHookBuildTriggerType {
			if !hmac.Equal([]byte(trigger.GitHubWebHook.Secret), []byte(secret)) {
				continue
			}
			return trigger.GitHubWebHook, nil
		}
	}
	return nil, ErrSecretMismatch
}

// NewWarning returns an StatusError object with a http.StatusOK (200) code.
func NewWarning(message string) *kerrors.StatusError {
	return &kerrors.StatusError{ErrStatus: unversioned.Status{
		Status:  unversioned.StatusSuccess,
		Code:    http.StatusOK,
		Message: message,
	}}
}
