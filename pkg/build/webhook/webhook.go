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
	Extract(buildCfg *buildapi.BuildConfig, secret, path string, req *http.Request) (*buildapi.SourceRevision, []kapi.EnvVar, *buildapi.DockerStrategyOptions, bool, error)
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
	buildTriggers = buildapi.FindTriggerPolicy(triggerType, config)
	if len(buildTriggers) == 0 {
		err = ErrHookNotEnabled
	}
	return buildTriggers, err
}

// ValidateWebHookSecret validates the provided secret against all currently
// defined webhook secrets and if it is valid, returns its information.
func ValidateWebHookSecret(webHookTriggers []buildapi.BuildTriggerPolicy, secret string) (*buildapi.WebHookTrigger, error) {
	for _, trigger := range webHookTriggers {
		switch trigger.Type {
		case buildapi.GenericWebHookBuildTriggerType:
			if !hmac.Equal([]byte(trigger.GenericWebHook.Secret), []byte(secret)) {
				continue
			}
			return trigger.GenericWebHook, nil
		case buildapi.GitHubWebHookBuildTriggerType:
			if !hmac.Equal([]byte(trigger.GitHubWebHook.Secret), []byte(secret)) {
				continue
			}
			return trigger.GitHubWebHook, nil

		case buildapi.GitLabWebHookBuildTriggerType:
			if !hmac.Equal([]byte(trigger.GitLabWebHook.Secret), []byte(secret)) {
				continue
			}
			return trigger.GitLabWebHook, nil

		case buildapi.BitbucketWebHookBuildTriggerType:
			if !hmac.Equal([]byte(trigger.BitbucketWebHook.Secret), []byte(secret)) {
				continue
			}
			return trigger.BitbucketWebHook, nil
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
