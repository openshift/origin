package github

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/golang/glog"
	"github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/build/webhook"
)

// WebHook used for processing github webhook requests.
type WebHook struct{}

// New returns github webhook plugin.
func New() *WebHook {
	return &WebHook{}
}

type commit struct {
	ID        string                `json:"id,omitempty"`
	Author    api.SourceControlUser `json:"author,omitempty"`
	Committer api.SourceControlUser `json:"committer,omitempty"`
	Message   string                `json:"message,omitempty"`
}

type pushEvent struct {
	Ref        string `json:"ref,omitempty"`
	After      string `json:"after,omitempty"`
	HeadCommit commit `json:"head_commit,omitempty"`
}

// Extract services webhooks from github.com
func (p *WebHook) Extract(buildCfg *api.BuildConfig, secret, path string, req *http.Request) (revision *api.SourceRevision, proceed bool, err error) {
	trigger, ok := webhook.FindTriggerPolicy(api.GitHubWebHookBuildTriggerType, buildCfg)
	if !ok {
		err = webhook.ErrHookNotEnabled
		return
	}
	glog.V(4).Infof("Checking if the provided secret for BuildConfig %s/%s matches", buildCfg.Namespace, buildCfg.Name)
	if trigger.GitHubWebHook.Secret != secret {
		err = webhook.ErrSecretMismatch
		return
	}
	glog.V(4).Infof("Verifying build request for BuildConfig %s/%s", buildCfg.Namespace, buildCfg.Name)
	if err = verifyRequest(req); err != nil {
		return
	}
	method := getEvent(req.Header)
	if method != "ping" && method != "push" {
		err = fmt.Errorf("unknown X-GitHub-Event or X-Gogs-Event %s", method)
		return
	}
	if method == "ping" {
		proceed = false
		return
	}
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return
	}
	var event pushEvent
	if err = json.Unmarshal(body, &event); err != nil {
		return
	}
	proceed = webhook.GitRefMatches(event.Ref, buildCfg.Spec.Source.Git.Ref)
	if !proceed {
		glog.V(2).Infof("Skipping build for BuildConfig %s/%s.  Branch reference from '%s' does not match configuration", buildCfg.Namespace, buildCfg, event)
	}

	revision = &api.SourceRevision{
		Type: api.BuildSourceGit,
		Git: &api.GitSourceRevision{
			Commit:    event.HeadCommit.ID,
			Author:    event.HeadCommit.Author,
			Committer: event.HeadCommit.Committer,
			Message:   event.HeadCommit.Message,
		},
	}

	return
}

func verifyRequest(req *http.Request) error {
	if method := req.Method; method != "POST" {
		return fmt.Errorf("unsupported HTTP method %s", method)
	}
	if contentType := req.Header.Get("Content-Type"); contentType != "application/json" {
		return fmt.Errorf("unsupported Content-Type %s", contentType)
	}
	if len(getEvent(req.Header)) == 0 {
		return errors.New("missing X-GitHub-Event or X-Gogs-Event")
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
