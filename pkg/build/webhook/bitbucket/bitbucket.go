package bitbucket

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"mime"
	"net/http"

	"k8s.io/client-go/pkg/api/errors"
	kapi "k8s.io/kubernetes/pkg/api"

	"github.com/golang/glog"
	buildapi "github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/build/webhook"
)

// WebHook used for processing gitlab webhook requests.
type WebHook struct{}

// New returns gitlab webhook plugin.
func New() *WebHook {
	return &WebHook{}
}

// A push event for Bitbucket webhooks. Only some json parameters are used. The
// Bitbucket payload is less flat than GitLab or GitHub
// More information on Bitbucket push events here:
// https://confluence.atlassian.com/bitbucket/event-payloads-740262817.html#EventPayloads-Push
type pushEvent struct {
	Actor user `json:"actor"`
	Push  push `json:"push"`
}

type user struct {
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
}

type push struct {
	Changes []change `json:"changes"`
}

type change struct {
	Commits []commit `json:"commits"`
	New     new      `json:"new"`
	Old     old      `json:"old"`
}

type commit struct {
	Hash    string `json:"hash"`
	Message string `json:"message"`
	Author  user   `json:"author"`
}

type new struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

type old struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

// Extract services webhooks from bitbucket.com
func (p *WebHook) Extract(buildCfg *buildapi.BuildConfig, secret, path string, req *http.Request) (revision *buildapi.SourceRevision, envvars []kapi.EnvVar, dockerStrategyOptions *buildapi.DockerStrategyOptions, proceed bool, err error) {
	triggers, err := webhook.FindTriggerPolicy(buildapi.BitbucketWebHookBuildTriggerType, buildCfg)
	if err != nil {
		return revision, envvars, dockerStrategyOptions, false, err
	}

	glog.V(4).Infof("Checking if the provided secret for BuildConfig %s/%s matches", buildCfg.Namespace, buildCfg.Name)
	if _, err = webhook.ValidateWebHookSecret(triggers, secret); err != nil {
		return revision, envvars, dockerStrategyOptions, false, err
	}

	glog.V(4).Infof("Verifying build request for BuildConfig %s/%s", buildCfg.Namespace, buildCfg.Name)
	if err = verifyRequest(req); err != nil {
		return revision, envvars, dockerStrategyOptions, false, err
	}

	method := getEvent(req.Header)
	if method != "repo:push" {
		return revision, envvars, dockerStrategyOptions, false, errors.NewBadRequest(fmt.Sprintf("Unknown Bitbucket X-Event-Key %s", method))
	}

	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return revision, envvars, dockerStrategyOptions, false, errors.NewBadRequest(err.Error())
	}

	var event pushEvent
	if err = json.Unmarshal(body, &event); err != nil {
		return revision, envvars, dockerStrategyOptions, false, errors.NewBadRequest(err.Error())
	}

	// We use old here specifically. If the branch is deleted in a push, the New
	// object will be nil.
	if !webhook.GitRefMatches(event.Push.Changes[0].Old.Name, webhook.DefaultConfigRef, &buildCfg.Spec.Source) {
		glog.V(2).Infof("Skipping build for BuildConfig %s/%s.  Branch reference from '%s' does not match configuration", buildCfg.Namespace, buildCfg, event)
		return revision, envvars, dockerStrategyOptions, false, err
	}

	lastCommit := event.Push.Changes[0].Commits[0]
	author := buildapi.SourceControlUser{
		Name: lastCommit.Author.Username,
	}

	revision = &buildapi.SourceRevision{
		Git: &buildapi.GitSourceRevision{
			Commit:    lastCommit.Hash,
			Author:    author,
			Committer: author,
			Message:   lastCommit.Message,
		},
	}
	return revision, envvars, dockerStrategyOptions, true, err
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
		return errors.NewBadRequest("missing X-Event-Key")
	}
	return nil
}

func getEvent(header http.Header) string {
	return header.Get("X-Event-Key")
}
