package generic

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"mime"
	"net/http"

	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/yaml"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	"github.com/openshift/origin/pkg/build/webhook"
)

// WebHookPlugin used for processing manual(or other) webhook requests.
type WebHookPlugin struct{}

// New returns a generic webhook plugin.
func New() *WebHookPlugin {
	return &WebHookPlugin{}
}

// Extract services generic webhooks.
func (p *WebHookPlugin) Extract(buildCfg *buildapi.BuildConfig, trigger *buildapi.WebHookTrigger, req *http.Request) (revision *buildapi.SourceRevision, envvars []kapi.EnvVar, dockerStrategyOptions *buildapi.DockerStrategyOptions, proceed bool, err error) {
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

	var data buildapi.GenericWebHookEvent
	if contentType == "application/yaml" {
		body, err = yaml.ToJSON(body)
		if err != nil {
			warning := webhook.NewWarning(fmt.Sprintf("error converting payload to json: %v, ignoring payload and continuing with build", err))
			return revision, envvars, dockerStrategyOptions, true, warning
		}
	}
	if err = json.Unmarshal(body, &data); err != nil {
		warning := webhook.NewWarning(fmt.Sprintf("error unmarshalling payload: %v, ignoring payload and continuing with build", err))
		return revision, envvars, dockerStrategyOptions, true, warning
	}
	if len(data.Env) > 0 && trigger.AllowEnv {
		envvars = data.Env
	}
	if data.DockerStrategyOptions != nil {
		dockerStrategyOptions = data.DockerStrategyOptions
	}
	if buildCfg.Spec.Source.Git == nil {
		// everything below here is specific to git-based builds
		return revision, envvars, dockerStrategyOptions, true, nil
	}
	if data.Git == nil {
		warning := webhook.NewWarning("no git information found in payload, ignoring and continuing with build")
		return revision, envvars, dockerStrategyOptions, true, warning
	}

	if data.Git.Refs != nil {
		for _, ref := range data.Git.Refs {
			if webhook.GitRefMatches(ref.Ref, webhook.DefaultConfigRef, &buildCfg.Spec.Source) {
				revision = &buildapi.SourceRevision{
					Git: &ref.GitSourceRevision,
				}
				return revision, envvars, dockerStrategyOptions, true, nil
			}
		}
		warning := webhook.NewWarning(fmt.Sprintf("skipping build. None of the supplied refs matched %q", buildCfg.Spec.Source.Git.Ref))
		return revision, envvars, dockerStrategyOptions, false, warning
	}
	if !webhook.GitRefMatches(data.Git.Ref, webhook.DefaultConfigRef, &buildCfg.Spec.Source) {
		warning := webhook.NewWarning(fmt.Sprintf("skipping build. Branch reference from %q does not match configuration", data.Git.Ref))
		return revision, envvars, dockerStrategyOptions, false, warning
	}
	revision = &buildapi.SourceRevision{
		Git: &data.Git.GitSourceRevision,
	}
	return revision, envvars, dockerStrategyOptions, true, nil
}

// GetTriggers retrieves the WebHookTriggers for this webhook type (if any)
func (p *WebHookPlugin) GetTriggers(buildConfig *buildapi.BuildConfig) ([]*buildapi.WebHookTrigger, error) {
	triggers := buildapi.FindTriggerPolicy(buildapi.GenericWebHookBuildTriggerType, buildConfig)
	webhookTriggers := []*buildapi.WebHookTrigger{}
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
