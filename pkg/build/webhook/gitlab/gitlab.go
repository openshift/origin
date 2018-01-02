package gitlab

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

// WebHookPlugin used for processing gitlab webhook requests.
type WebHookPlugin struct{}

// New returns gitlab webhook plugin.
func New() *WebHookPlugin {
	return &WebHookPlugin{}
}

// NOTE - unlike github, there is no separate commiter, just the author
type commit struct {
	ID      string                     `json:"id,omitempty"`
	Author  buildapi.SourceControlUser `json:"author,omitempty"`
	Message string                     `json:"message,omitempty"`
}

// NOTE - unlike github, the head commit is not highlighted ... only the commit array is provided,
// where the last commit is the latest commit
type pushEvent struct {
	Ref     string   `json:"ref,omitempty"`
	After   string   `json:"after,omitempty"`
	Commits []commit `json:"commits,omitempty"`
}

// Extract services webhooks from GitLab server
func (p *WebHookPlugin) Extract(buildCfg *buildapi.BuildConfig, trigger *buildapi.WebHookTrigger, req *http.Request) (revision *buildapi.SourceRevision, envvars []kapi.EnvVar, dockerStrategyOptions *buildapi.DockerStrategyOptions, proceed bool, err error) {
	glog.V(4).Infof("Verifying build request for BuildConfig %s/%s", buildCfg.Namespace, buildCfg.Name)
	if err = verifyRequest(req); err != nil {
		return revision, envvars, dockerStrategyOptions, proceed, err
	}
	method := getEvent(req.Header)
	if method != "Push Hook" {
		return revision, envvars, dockerStrategyOptions, proceed, errors.NewBadRequest(fmt.Sprintf("Unknown X-Gitlab-Event %s", method))
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

	lastCommit := event.Commits[len(event.Commits)-1]

	revision = &buildapi.SourceRevision{
		Git: &buildapi.GitSourceRevision{
			Commit:    lastCommit.ID,
			Author:    lastCommit.Author,
			Committer: lastCommit.Author,
			Message:   lastCommit.Message,
		},
	}
	return revision, envvars, dockerStrategyOptions, true, err
}

// GetTriggers retrieves the WebHookTriggers for this webhook type (if any)
func (p *WebHookPlugin) GetTriggers(buildConfig *buildapi.BuildConfig) ([]*buildapi.WebHookTrigger, error) {
	triggers := buildapi.FindTriggerPolicy(buildapi.GitLabWebHookBuildTriggerType, buildConfig)
	webhookTriggers := []*buildapi.WebHookTrigger{}
	for _, trigger := range triggers {
		if trigger.GitLabWebHook != nil {
			webhookTriggers = append(webhookTriggers, trigger.GitLabWebHook)
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
		return errors.NewBadRequest("missing X-Gitlab-Event")
	}
	return nil
}

func getEvent(header http.Header) string {
	return header.Get("X-Gitlab-Event")
}
