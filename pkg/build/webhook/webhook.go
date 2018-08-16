package webhook

import (
	"crypto/hmac"
	"errors"
	"net/http"
	"strings"

	"github.com/golang/glog"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubernetes "k8s.io/client-go/kubernetes/typed/core/v1"

	buildv1 "github.com/openshift/api/build/v1"
	buildutil "github.com/openshift/origin/pkg/build/util"
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
	Extract(buildCfg *buildv1.BuildConfig, trigger *buildv1.WebHookTrigger, req *http.Request) (*buildv1.SourceRevision, []corev1.EnvVar, *buildv1.DockerStrategyOptions, bool, error)
	GetTriggers(buildConfig *buildv1.BuildConfig) ([]*buildv1.WebHookTrigger, error)
}

// GitRefMatches determines if the ref from a webhook event matches a build
// configuration
func GitRefMatches(eventRef, configRef string, buildSource *buildv1.BuildSource) bool {
	if buildSource.Git != nil && len(buildSource.Git.Ref) != 0 {
		configRef = buildSource.Git.Ref
	}

	eventRef = strings.TrimPrefix(eventRef, refPrefix)
	configRef = strings.TrimPrefix(configRef, refPrefix)
	return configRef == eventRef
}

// NewWarning returns an StatusError object with a http.StatusOK (200) code.
func NewWarning(message string) *kerrors.StatusError {
	return &kerrors.StatusError{ErrStatus: metav1.Status{
		Status:  metav1.StatusSuccess,
		Code:    http.StatusOK,
		Message: message,
	}}
}

// CheckSecret tests the user provided secret against the secrets for the webhook triggers, if a match is found
// then the corresponding webhook trigger is returned.
func CheckSecret(namespace, userSecret string, triggers []*buildv1.WebHookTrigger, secretsClient kubernetes.SecretsGetter) (*buildv1.WebHookTrigger, error) {
	for i := range triggers {
		secretRef := triggers[i].SecretReference
		secret := triggers[i].Secret
		if len(secret) > 0 {
			if hmac.Equal([]byte(secret), []byte(userSecret)) {
				return triggers[i], nil
			}
		}
		if secretRef != nil {
			glog.V(4).Infof("Checking user secret against secret ref %s", secretRef.Name)
			s, err := secretsClient.Secrets(namespace).Get(secretRef.Name, metav1.GetOptions{})
			if err != nil && !kerrors.IsNotFound(err) {
				return nil, err
			}
			if v, ok := s.Data[buildutil.WebHookSecretKey]; ok {
				if hmac.Equal(v, []byte(userSecret)) {
					return triggers[i], nil
				}
			}
		}
	}
	glog.V(4).Infof("did not find a matching secret")
	return nil, ErrSecretMismatch
}

func GenerateBuildTriggerInfo(revision *buildv1.SourceRevision, hookType string) (buildTriggerCauses []buildv1.BuildTriggerCause) {
	hiddenSecret := "<secret>"
	switch {
	case hookType == "generic":
		buildTriggerCauses = append(buildTriggerCauses,
			buildv1.BuildTriggerCause{
				Message: buildutil.BuildTriggerCauseGenericMsg,
				GenericWebHook: &buildv1.GenericWebHookCause{
					Revision: revision,
					Secret:   hiddenSecret,
				},
			})
	case hookType == "github":
		buildTriggerCauses = append(buildTriggerCauses,
			buildv1.BuildTriggerCause{
				Message: buildutil.BuildTriggerCauseGithubMsg,
				GitHubWebHook: &buildv1.GitHubWebHookCause{
					Revision: revision,
					Secret:   hiddenSecret,
				},
			})
	case hookType == "gitlab":
		buildTriggerCauses = append(buildTriggerCauses,
			buildv1.BuildTriggerCause{
				Message: buildutil.BuildTriggerCauseGitLabMsg,
				GitLabWebHook: &buildv1.GitLabWebHookCause{
					CommonWebHookCause: buildv1.CommonWebHookCause{
						Revision: revision,
						Secret:   hiddenSecret,
					},
				},
			})
	case hookType == "bitbucket":
		buildTriggerCauses = append(buildTriggerCauses,
			buildv1.BuildTriggerCause{
				Message: buildutil.BuildTriggerCauseBitbucketMsg,
				BitbucketWebHook: &buildv1.BitbucketWebHookCause{
					CommonWebHookCause: buildv1.CommonWebHookCause{
						Revision: revision,
						Secret:   hiddenSecret,
					},
				},
			})
	}
	return buildTriggerCauses
}
