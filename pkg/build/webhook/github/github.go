package github

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"mime"
	"net/http"

	"k8s.io/apimachinery/pkg/api/errors"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	"github.com/golang/glog"
	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	"github.com/openshift/origin/pkg/build/webhook"
)

// WebHookPlugin used for processing github webhook requests.
type WebHookPlugin struct{}

// New returns github webhook plugin.
func New() *WebHookPlugin {
	return &WebHookPlugin{}
}

type commit struct {
	ID        string                     `json:"id,omitempty"`
	Author    buildapi.SourceControlUser `json:"author,omitempty"`
	Committer buildapi.SourceControlUser `json:"committer,omitempty"`
	Message   string                     `json:"message,omitempty"`
}

type pushEvent struct {
	Ref        string `json:"ref,omitempty"`
	After      string `json:"after,omitempty"`
	HeadCommit commit `json:"head_commit,omitempty"`
}

// Extract services webhooks from github.com
func (p *WebHookPlugin) Extract(buildCfg *buildapi.BuildConfig, trigger *buildapi.WebHookTrigger, req *http.Request) (revision *buildapi.SourceRevision, envvars []kapi.EnvVar, dockerStrategyOptions *buildapi.DockerStrategyOptions, proceed bool, err error) {
	glog.V(4).Infof("Verifying build request for BuildConfig %s/%s", buildCfg.Namespace, buildCfg.Name)
	if err = verifyRequest(req); err != nil {
		return revision, envvars, dockerStrategyOptions, proceed, err
	}
	method := getEvent(req.Header)
	if method != "ping" && method != "push" {
		return revision, envvars, dockerStrategyOptions, proceed, errors.NewBadRequest(fmt.Sprintf("Unknown X-GitHub-Event or X-Gogs-Event %s", method))
	}
	if method == "ping" {
		return revision, envvars, dockerStrategyOptions, proceed, err
	}
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return revision, envvars, dockerStrategyOptions, proceed, errors.NewBadRequest(err.Error())
	}
	var event pushEvent
	if err = json.Unmarshal(body, &event); err != nil {
		return revision, envvars, dockerStrategyOptions, proceed, errors.NewBadRequest(err.Error())
	}
	if !webhook.GitRefMatches(event.Ref, webhook.DefaultConfigRef, &buildCfg.Spec.Source) {
		glog.V(2).Infof("Skipping build for BuildConfig %s/%s.  Branch reference from '%s' does not match configuration", buildCfg.Namespace, buildCfg.Name, event)
		return revision, envvars, dockerStrategyOptions, proceed, err
	}

	revision = &buildapi.SourceRevision{
		Git: &buildapi.GitSourceRevision{
			Commit:    event.HeadCommit.ID,
			Author:    event.HeadCommit.Author,
			Committer: event.HeadCommit.Committer,
			Message:   event.HeadCommit.Message,
		},
	}
	return revision, envvars, dockerStrategyOptions, true, err
}

// GetTriggers retrieves the WebHookTriggers for this webhook type (if any)
func (p *WebHookPlugin) GetTriggers(buildConfig *buildapi.BuildConfig) ([]*buildapi.WebHookTrigger, error) {
	triggers := buildapi.FindTriggerPolicy(buildapi.GitHubWebHookBuildTriggerType, buildConfig)
	webhookTriggers := []*buildapi.WebHookTrigger{}
	for _, trigger := range triggers {
		if trigger.GitHubWebHook != nil {
			webhookTriggers = append(webhookTriggers, trigger.GitHubWebHook)
		}
	}
	if len(webhookTriggers) == 0 {
		return nil, webhook.ErrHookNotEnabled
	}
	return webhookTriggers, nil
}

func verifyRequest(req *http.Request) error {
	if method := req.Method; method != "POST" {
		return webhook.MethodNotSupported
	}
	contentType := req.Header.Get("Content-Type")
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return errors.NewBadRequest(fmt.Sprintf("non-parseable Content-Type %s (%s)", contentType, err))
	}
	if mediaType != "application/json" {
		return errors.NewBadRequest(fmt.Sprintf("unsupported Content-Type %s", contentType))
	}
	if len(getEvent(req.Header)) == 0 {
		return errors.NewBadRequest("missing X-GitHub-Event or X-Gogs-Event")
	}
	return nil
}

func getEvent(header http.Header) string {
	event := header.Get("X-GitHub-Event")
	if len(event) == 0 {
		event = header.Get("X-Gogs-Event")
	}
	return event
}
