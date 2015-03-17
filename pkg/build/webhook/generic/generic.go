package generic

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/golang/glog"

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
func (p *WebHookPlugin) Extract(buildCfg *api.BuildConfig, secret, path string, req *http.Request) (revision *api.SourceRevision, proceed bool, err error) {
	trigger, ok := webhook.FindTriggerPolicy(api.GenericWebHookBuildTriggerType, buildCfg)
	if !ok {
		err = fmt.Errorf("BuildConfig %s does not support the Generic webhook trigger type", buildCfg.Name)
		return
	}
	if trigger.GenericWebHook.Secret != secret {
		err = fmt.Errorf("Secret does not match for BuildConfig %s", buildCfg.Name)
		return
	}
	if err = verifyRequest(req); err != nil {
		return
	}
	if req.Body != nil {
		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			return nil, false, err
		}
		if len(body) == 0 {
			return nil, true, nil
		}
		var data api.GenericWebHookEvent
		if err = json.Unmarshal(body, &data); err != nil {
			glog.V(4).Infof("Error unmarshaling json %v, but continuing", err)
			return nil, true, nil
		}
		if !webhook.GitRefMatches(data.Git.Ref, buildCfg.Parameters.Source.Git.Ref) {
			glog.V(2).Infof("Skipping build for '%s'.  Branch reference from '%s' does not match configuration", buildCfg, data)
			return nil, false, nil
		}
		revision = &api.SourceRevision{
			Type: api.BuildSourceGit,
			Git: &api.GitSourceRevision{
				Commit:    data.Git.Commit,
				Message:   data.Git.Message,
				Author:    data.Git.Author,
				Committer: data.Git.Committer,
			},
		}
	}
	return revision, true, nil
}

func verifyRequest(req *http.Request) error {
	if method := req.Method; method != "POST" {
		return fmt.Errorf("Unsupported HTTP method %s", method)
	}
	if userAgent := req.Header.Get("User-Agent"); len(strings.TrimSpace(userAgent)) == 0 {
		return fmt.Errorf("User-Agent must be populated with a non-empty value")
	}
	if contentLength := req.Header.Get("Content-Length"); strings.TrimSpace(contentLength) != "" {
		if contentType := req.Header.Get("Content-Type"); contentType != "application/json" {
			return fmt.Errorf("Unsupported Content-Type %s", contentType)
		}
	}
	return nil
}
