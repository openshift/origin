package github

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"mime"
	"net/http"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"

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
func (p *WebHook) Extract(buildCfg *api.BuildConfig, secret, path string, req *http.Request) (revision *api.SourceRevision, envvars []kapi.EnvVar, proceed bool, err error) {
	triggers, err := webhook.FindTriggerPolicy(api.GitHubWebHookBuildTriggerType, buildCfg)
	if err != nil {
		return revision, envvars, proceed, err
	}
	glog.V(4).Infof("Checking if the provided secret for BuildConfig %s/%s matches", buildCfg.Namespace, buildCfg.Name)

	if _, err = webhook.ValidateWebHookSecret(triggers, secret); err != nil {
		return revision, envvars, proceed, err
	}

	glog.V(4).Infof("Verifying build request for BuildConfig %s/%s", buildCfg.Namespace, buildCfg.Name)
	if err = verifyRequest(req); err != nil {
		return revision, envvars, proceed, err
	}
	method := getEvent(req.Header)
	if method != "ping" && method != "push" && method != "Push Hook" {
		return revision, envvars, proceed, errors.NewBadRequest(fmt.Sprintf("Unknown X-GitHub-Event, X-Gogs-Event or X-Gitlab-Event %s", method))
	}
	if method == "ping" {
		return revision, envvars, proceed, err
	}
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return revision, envvars, proceed, errors.NewBadRequest(err.Error())
	}
	var event pushEvent
	if err = json.Unmarshal(body, &event); err != nil {
		return revision, envvars, proceed, errors.NewBadRequest(err.Error())
	}
	if !webhook.GitRefMatches(event.Ref, webhook.DefaultConfigRef, &buildCfg.Spec.Source) {
		glog.V(2).Infof("Skipping build for BuildConfig %s/%s.  Branch reference from '%s' does not match configuration", buildCfg.Namespace, buildCfg, event)
		return revision, envvars, proceed, err
	}

	revision = &api.SourceRevision{
		Git: &api.GitSourceRevision{
			Commit:    event.HeadCommit.ID,
			Author:    event.HeadCommit.Author,
			Committer: event.HeadCommit.Committer,
			Message:   event.HeadCommit.Message,
		},
	}
	return revision, envvars, true, err
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
		return errors.NewBadRequest("missing X-GitHub-Event, X-Gogs-Event or X-Gitlab-Event")
	}
	return nil
}

func getEvent(header http.Header) string {
	event := header.Get("X-GitHub-Event")
	if len(event) == 0 {
		event = header.Get("X-Gogs-Event")
	}
	if len(event) == 0 {
		event = header.Get("X-Gitlab-Event")
	}
	return event
}
