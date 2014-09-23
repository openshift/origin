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
	proceed = (method == "push")
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return
	}
	var data map[string]interface{}
	if err = json.Unmarshal(body, &data); err != nil {
		return
	}

	return
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
