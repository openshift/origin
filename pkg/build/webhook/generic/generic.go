package generic

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"mime"
	"net/http"

	"github.com/golang/glog"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/util/yaml"

	"github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/build/webhook"
)

// WebHookPlugin used for processing manual(or other) webhook requests.
type WebHookPlugin struct{}

// New returns a generic webhook plugin.
func New() *WebHookPlugin {
	return &WebHookPlugin{}
}

// Extract services generic webhooks.
func (p *WebHookPlugin) Extract(buildCfg *api.BuildConfig, secret, path string, req *http.Request) (revision *api.SourceRevision, envvars []kapi.EnvVar, proceed bool, err error) {
	triggers, err := webhook.FindTriggerPolicy(api.GenericWebHookBuildTriggerType, buildCfg)
	if err != nil {
		return revision, envvars, false, err
	}
	glog.V(4).Infof("Checking if the provided secret for BuildConfig %s/%s matches", buildCfg.Namespace, buildCfg.Name)

	trigger, err := webhook.ValidateWebHookSecret(triggers, secret)
	if err != nil {
		return revision, envvars, false, err
	}

	glog.V(4).Infof("Verifying build request for BuildConfig %s/%s", buildCfg.Namespace, buildCfg.Name)
	if err = verifyRequest(req); err != nil {
		return revision, envvars, false, err
	}

	if buildCfg.Spec.Source.Git == nil {
		glog.V(4).Infof("No git source defined for BuildConfig %s/%s, but triggering anyway", buildCfg.Namespace, buildCfg.Name)
		return revision, envvars, true, err
	}

	contentType := req.Header.Get("Content-Type")
	if len(contentType) != 0 {
		contentType, _, err = mime.ParseMediaType(contentType)
		if err != nil {
			return nil, envvars, false, fmt.Errorf("non-parseable Content-Type %s (%s)", contentType, err)
		}
	}

	if req.Body != nil && (contentType == "application/json" || contentType == "application/yaml") {
		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			return nil, envvars, false, err
		}

		if len(body) == 0 {
			return nil, envvars, true, nil
		}

		var data api.GenericWebHookEvent
		if contentType == "application/yaml" {
			body, err = yaml.ToJSON(body)
			if err != nil {
				glog.V(4).Infof("Error converting payload to json %v, but continuing with build", err)
				return nil, envvars, true, nil
			}
		}
		if err = json.Unmarshal(body, &data); err != nil {
			glog.V(4).Infof("Error unmarshalling payload %v, but continuing with build", err)
			return nil, envvars, true, nil
		}
		if len(data.Env) > 0 && trigger.AllowEnv {
			envvars = data.Env
		}
		if data.Git == nil {
			glog.V(4).Infof("No git information for the generic webhook found in %s/%s", buildCfg.Namespace, buildCfg.Name)
			return nil, envvars, true, nil
		}

		if data.Git.Refs != nil {
			for _, ref := range data.Git.Refs {
				if webhook.GitRefMatches(ref.Ref, webhook.DefaultConfigRef, &buildCfg.Spec.Source) {
					revision = &api.SourceRevision{
						Git: &ref.GitSourceRevision,
					}
					return revision, envvars, true, nil
				}
			}
			glog.V(2).Infof("Skipping build for BuildConfig %s/%s. None of the supplied refs matched %q", buildCfg.Namespace, buildCfg, buildCfg.Spec.Source.Git.Ref)
			return nil, envvars, false, nil
		}
		if !webhook.GitRefMatches(data.Git.Ref, webhook.DefaultConfigRef, &buildCfg.Spec.Source) {
			glog.V(2).Infof("Skipping build for BuildConfig %s/%s. Branch reference from %q does not match configuration", buildCfg.Namespace, buildCfg.Name, data.Git.Ref)
			return nil, envvars, false, nil
		}
		revision = &api.SourceRevision{
			Git: &data.Git.GitSourceRevision,
		}
	}
	return revision, envvars, true, nil
}

func verifyRequest(req *http.Request) error {
	if method := req.Method; method != "POST" {
		return fmt.Errorf("Unsupported HTTP method %s", method)
	}
	return nil
}
