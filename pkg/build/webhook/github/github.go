package github

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/openshift/origin/pkg/build/api"
)

// GitHubWebHook used for processing github webhook requests.
type GitHubWebHook struct{}

// New returns github webhook plugin.
func New() *GitHubWebHook {
	return &GitHubWebHook{}
}

type gitHubCommit struct {
	ID        string                `json:"id,omitempty" yaml:"id,omitempty"`
	Author    api.SourceControlUser `json:"author,omitempty" yaml:"author,omitempty"`
	Committer api.SourceControlUser `json:"committer,omitempty" yaml:"committer,omitempty"`
	Message   string                `json:"message,omitempty" yaml:"message,omitempty"`
}

type gitHubPushEvent struct {
	Ref        string       `json:"ref,omitempty" yaml:"ref,omitempty"`
	After      string       `json:"after,omitempty" yaml:"after,omitempty"`
	HeadCommit gitHubCommit `json:"head_commit,omitempty" yaml:"head_commit,omitempty"`
}

// Extract responsible for servicing webhooks from github.com.
func (p *GitHubWebHook) Extract(buildCfg *api.BuildConfig, path string, req *http.Request) (build *api.Build, proceed bool, err error) {
	if err = verifyRequest(req); err != nil {
		return
	}
	method := req.Header.Get("X-GitHub-Event")
	if method != "ping" && method != "push" {
		err = fmt.Errorf("Unknown X-GitHub-Event %s", method)
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
	var event gitHubPushEvent
	if err = json.Unmarshal(body, &event); err != nil {
		return
	}
	proceed = buildConfigRefMatches(event, buildCfg)

	build = &api.Build{
		Source: buildCfg.Source,
		Revision: api.SourceRevision{
			Type: api.BuildSourceGit,
			Git: &api.GitSourceRevision{
				Commit:    event.HeadCommit.ID,
				Author:    event.HeadCommit.Author,
				Committer: event.HeadCommit.Committer,
				Message:   event.HeadCommit.Message,
			},
		},
		Input: buildCfg.DesiredInput,
	}

	return
}

func buildConfigRefMatches(event gitHubPushEvent, buildCfg *api.BuildConfig) bool {
	const RefPrefix = "refs/heads/"
	eventRef := strings.TrimPrefix(event.Ref, RefPrefix)
	configRef := strings.TrimPrefix(buildCfg.Source.Git.Ref, RefPrefix)
	if configRef == "" {
		configRef = "master"
	}
	return configRef == eventRef
}

func verifyRequest(req *http.Request) error {
	if method := req.Method; method != "POST" {
		return fmt.Errorf("Unsupported HTTP method %s", method)
	}
	if contentType := req.Header.Get("Content-Type"); contentType != "application/json" {
		return fmt.Errorf("Unsupported Content-Type %s", contentType)
	}
	if userAgent := req.Header.Get("User-Agent"); !strings.HasPrefix(userAgent, "GitHub-Hookshot/") {
		return fmt.Errorf("Unsupported User-Agent %s")
	}
	if req.Header.Get("X-GitHub-Event") == "" {
		return errors.New("Missing X-GitHub-Event")
	}
	return nil
}
