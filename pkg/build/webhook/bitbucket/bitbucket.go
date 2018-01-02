package bitbucket

import (
	"encoding/json"
	"fmt"
	"io"
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

// A push event for Bitbucket webhooks. Only some json parameters are used. The
// Bitbucket payload is less flat than GitLab or GitHub
// More information on Bitbucket push events here:
// https://confluence.atlassian.com/bitbucket/event-payloads-740262817.html#EventPayloads-Push
type pushEvent struct {
	Push push `json:"push"`
}

type push struct {
	Changes []change `json:"changes"`
}

type change struct {
	Commits []commit `json:"commits"`
	Old     info     `json:"old"`
}

type commit struct {
	Hash    string `json:"hash"`
	Message string `json:"message"`
	Author  user   `json:"author"`
}

type user struct {
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
}

type info struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

type pushEvent54 struct {
	Changes []change54 `json:"changes"`
}

type change54 struct {
	Ref    ref    `json:"ref"`
	ToHash string `json:"toHash"`
}

type ref struct {
	DisplayID string `json:"displayId"`
}

// Extract services webhooks from bitbucket.com
func (p *WebHookPlugin) Extract(buildCfg *buildapi.BuildConfig, trigger *buildapi.WebHookTrigger, req *http.Request) (revision *buildapi.SourceRevision, envvars []kapi.EnvVar, dockerStrategyOptions *buildapi.DockerStrategyOptions, proceed bool, err error) {
	glog.V(4).Infof("Verifying build request for BuildConfig %s/%s", buildCfg.Namespace, buildCfg.Name)
	if err = verifyRequest(req); err != nil {
		return revision, envvars, dockerStrategyOptions, false, err
	}

	method := getEvent(req.Header)
	branch := ""
	switch method {
	// https://confluence.atlassian.com/bitbucket/event-payloads-740262817.html
	case "repo:push":
		branch, revision, err = getInfoFromEvent(req.Body)
		if err != nil {
			return revision, envvars, dockerStrategyOptions, false, errors.NewBadRequest(err.Error())
		}

	// https://confluence.atlassian.com/bitbucketserver/event-payload-938025882.html
	case "repo:refs_changed":
		branch, revision, err = getInfoFromEvent54(req.Body)
		if err != nil {
			return revision, envvars, dockerStrategyOptions, false, errors.NewBadRequest(err.Error())
		}
	default:
		return revision, envvars, dockerStrategyOptions, false, errors.NewBadRequest(fmt.Sprintf("Unknown Bitbucket X-Event-Key %s", method))
	}

	if !webhook.GitRefMatches(branch, webhook.DefaultConfigRef, &buildCfg.Spec.Source) {
		glog.V(2).Infof("Skipping build for BuildConfig %s/%s.  Branch reference '%s' does not match configuration", buildCfg.Namespace, buildCfg.Name, branch)
		return revision, envvars, dockerStrategyOptions, false, err
	}

	return revision, envvars, dockerStrategyOptions, true, err
}

// GetTriggers retrieves the WebHookTriggers for this webhook type (if any)
func (p *WebHookPlugin) GetTriggers(buildConfig *buildapi.BuildConfig) ([]*buildapi.WebHookTrigger, error) {
	triggers := buildapi.FindTriggerPolicy(buildapi.BitbucketWebHookBuildTriggerType, buildConfig)
	webhookTriggers := []*buildapi.WebHookTrigger{}
	for _, trigger := range triggers {
		if trigger.BitbucketWebHook != nil {
			webhookTriggers = append(webhookTriggers, trigger.BitbucketWebHook)
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
		return errors.NewBadRequest("missing X-Event-Key")
	}
	return nil
}

func getEvent(header http.Header) string {
	return header.Get("X-Event-Key")
}

func getInfoFromEvent(body io.ReadCloser) (string, *buildapi.SourceRevision, error) {
	data, err := ioutil.ReadAll(body)
	if err != nil {
		return "", nil, err
	}

	var event pushEvent
	if err = json.Unmarshal(data, &event); err != nil {
		return "", nil, err
	}
	if len(event.Push.Changes) == 0 {
		return "", nil, fmt.Errorf("Unable to extract valid event from payload: %s", string(data))
	}
	lastCommit := event.Push.Changes[0].Commits[0]
	author := buildapi.SourceControlUser{
		Name: lastCommit.Author.Username,
	}

	revision := &buildapi.SourceRevision{
		Git: &buildapi.GitSourceRevision{
			Commit:    lastCommit.Hash,
			Author:    author,
			Committer: author,
			Message:   lastCommit.Message,
		},
	}
	// We use old here specifically. If the branch is deleted in a push, the New
	// object will be nil.
	return event.Push.Changes[0].Old.Name, revision, nil
}

func getInfoFromEvent54(body io.ReadCloser) (string, *buildapi.SourceRevision, error) {
	data, err := ioutil.ReadAll(body)
	if err != nil {
		return "", nil, err
	}

	var event pushEvent54
	if err = json.Unmarshal(data, &event); err != nil {
		return "", nil, err
	}
	if len(event.Changes) == 0 {
		return "", nil, fmt.Errorf("Unable to extract valid event from payload: %s", string(data))
	}
	revision := &buildapi.SourceRevision{
		Git: &buildapi.GitSourceRevision{
			Commit: event.Changes[0].ToHash,
		},
	}
	return event.Changes[0].Ref.DisplayID, revision, nil
}
