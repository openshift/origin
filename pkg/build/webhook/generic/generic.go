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

// GenericWebHookPlugin used for processing manual(or other) webhook requests.
type GenericWebHookPlugin struct{}

// New returns a generic webhook plugin.
func New() *GenericWebHookPlugin {
	return &GenericWebHookPlugin{}
}

type genericWebHookEvent struct {
	Type api.BuildSourceType `json:"type,omitempty" yaml:"type,omitempty"`
	Git  *genericGitInfo     `json:"git,omitempty" yaml:"git,omitempty"`
}

type genericGitInfo struct {
	api.GitBuildSource
	api.GitSourceRevision
}

// Extract responsible for servicing generic webhooks
func (p *GenericWebHookPlugin) Extract(buildCfg *api.BuildConfig, path string, req *http.Request) (build *api.Build, proceed bool, err error) {
	if err = verifyRequest(req); err != nil {
		return
	}
	build = &api.Build{
		Parameters: buildCfg.Parameters,
	}
	if req.Body != nil {
		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			return nil, false, err
		}
		var data genericWebHookEvent
		if err = json.Unmarshal(body, &data); err != nil {
			return nil, false, err
		}
		if !webhook.GitRefMatches(data.Git.Ref, buildCfg.Parameters.Source.Git.Ref) {
			glog.V(2).Infof("Skipping build for '%s'.  Branch reference from '%s' does not match configuration", buildCfg, data)
			return nil, false, nil
		}
		build.Parameters.Revision = &api.SourceRevision{
			Type: api.BuildSourceGit,
			Git: &api.GitSourceRevision{
				Commit:    data.Git.Commit,
				Message:   data.Git.Message,
				Author:    data.Git.Author,
				Committer: data.Git.Committer,
			},
		}
	}
	return build, true, nil
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
