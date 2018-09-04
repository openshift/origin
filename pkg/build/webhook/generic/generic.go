package generic

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"mime"
	"net/http"

	"github.com/golang/glog"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/yaml"

	buildv1 "github.com/openshift/api/build/v1"
	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	"github.com/openshift/origin/pkg/build/buildapihelpers"
	"github.com/openshift/origin/pkg/build/webhook"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
)

// WebHookPlugin used for processing manual(or other) webhook requests.
type WebHookPlugin struct{}

// New returns a generic webhook plugin.
func New() *WebHookPlugin {
	return &WebHookPlugin{}
}

// Extract services generic webhooks.
func (p *WebHookPlugin) Extract(buildCfg *buildv1.BuildConfig, trigger *buildv1.WebHookTrigger, req *http.Request) (revision *buildv1.SourceRevision, envvars []corev1.EnvVar, dockerStrategyOptions *buildv1.DockerStrategyOptions, proceed bool, err error) {
	glog.V(4).Infof("Verifying build request for BuildConfig %s/%s", buildCfg.Namespace, buildCfg.Name)
	if err = verifyRequest(req); err != nil {
		return revision, envvars, dockerStrategyOptions, false, err
	}

	contentType := req.Header.Get("Content-Type")
	if len(contentType) != 0 {
		contentType, _, err = mime.ParseMediaType(contentType)
		if err != nil {
			return revision, envvars, dockerStrategyOptions, false, errors.NewBadRequest(fmt.Sprintf("error parsing Content-Type: %s", err))
		}
	}

	if req.Body == nil {
		return revision, envvars, dockerStrategyOptions, true, nil
	}

	if contentType != "application/json" && contentType != "application/yaml" {
		warning := webhook.NewWarning("invalid Content-Type on payload, ignoring payload and continuing with build")
		return revision, envvars, dockerStrategyOptions, true, warning
	}

	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return revision, envvars, dockerStrategyOptions, false, errors.NewBadRequest(err.Error())
	}

	if len(body) == 0 {
		return revision, envvars, dockerStrategyOptions, true, nil
	}

	internalData := &buildapi.GenericWebHookEvent{}
	versionedData := &buildv1.GenericWebHookEvent{}
	if contentType == "application/yaml" {
		body, err = yaml.ToJSON(body)
		if err != nil {
			warning := webhook.NewWarning(fmt.Sprintf("error converting payload to json: %v, ignoring payload and continuing with build", err))
			return revision, envvars, dockerStrategyOptions, true, warning
		}
	}
	if err = json.Unmarshal(body, &versionedData); err != nil {
		warning := webhook.NewWarning(fmt.Sprintf("error unmarshalling payload: %v, ignoring payload and continuing with build", err))
		return revision, envvars, dockerStrategyOptions, true, warning
	}
	if err := legacyscheme.Scheme.Convert(versionedData, internalData, nil); err != nil {
		return revision, envvars, dockerStrategyOptions, false, errors.NewBadRequest(err.Error())
	}

	if len(versionedData.Env) > 0 && trigger.AllowEnv {
		envvars = versionedData.Env
	}
	if internalData.DockerStrategyOptions != nil {
		dockerStrategyOptions = versionedData.DockerStrategyOptions
	}
	if buildCfg.Spec.Source.Git == nil {
		// everything below here is specific to git-based builds
		return revision, envvars, dockerStrategyOptions, true, nil
	}
	if internalData.Git == nil {
		warning := webhook.NewWarning("no git information found in payload, ignoring and continuing with build")
		return revision, envvars, dockerStrategyOptions, true, warning
	}

	if internalData.Git.Refs != nil {
		for _, ref := range versionedData.Git.Refs {
			if webhook.GitRefMatches(ref.Ref, webhook.DefaultConfigRef, &buildCfg.Spec.Source) {
				revision = &buildv1.SourceRevision{
					Git: &ref.GitSourceRevision,
				}
				return revision, envvars, dockerStrategyOptions, true, nil
			}
		}
		warning := webhook.NewWarning(fmt.Sprintf("skipping build. None of the supplied refs matched %q", buildCfg.Spec.Source.Git.Ref))
		return revision, envvars, dockerStrategyOptions, false, warning
	}
	if !webhook.GitRefMatches(internalData.Git.Ref, webhook.DefaultConfigRef, &buildCfg.Spec.Source) {
		warning := webhook.NewWarning(fmt.Sprintf("skipping build. Branch reference from %q does not match configuration", internalData.Git.Ref))
		return revision, envvars, dockerStrategyOptions, false, warning
	}
	revision = &buildv1.SourceRevision{
		Git: &versionedData.Git.GitSourceRevision,
	}
	return revision, envvars, dockerStrategyOptions, true, nil
}

// GetTriggers retrieves the WebHookTriggers for this webhook type (if any)
func (p *WebHookPlugin) GetTriggers(buildConfig *buildv1.BuildConfig) ([]*buildv1.WebHookTrigger, error) {
	triggers := buildapihelpers.FindTriggerPolicy(buildv1.GenericWebHookBuildTriggerType, buildConfig)
	webhookTriggers := []*buildv1.WebHookTrigger{}
	for _, trigger := range triggers {
		if trigger.GenericWebHook != nil {
			webhookTriggers = append(webhookTriggers, trigger.GenericWebHook)
		}
	}
	if len(webhookTriggers) == 0 {
		return nil, webhook.ErrHookNotEnabled
	}
	return webhookTriggers, nil
}

func verifyRequest(req *http.Request) error {
	if req.Method != "POST" {
		return webhook.MethodNotSupported
	}
	return nil
}
