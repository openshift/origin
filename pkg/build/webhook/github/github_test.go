package github

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	"github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/build/webhook"
	"github.com/openshift/origin/pkg/client"
)

type osClient struct {
	client.Fake
}

func (_ *osClient) GetBuildConfig(ctx kapi.Context, id string) (result *api.BuildConfig, err error) {
	return &api.BuildConfig{
		Secret: "secret101",
		Parameters: api.BuildParameters{
			Source: api.BuildSource{
				Type: api.BuildSourceGit,
				Git: &api.GitBuildSource{
					URI: "git://github.com/my/repo.git",
				},
			},
		},
	}, nil
}

func (_ *osClient) WatchBuilds(ctx kapi.Context, field, label labels.Selector, resourceVersion string) (watch.Interface, error) {
	return nil, nil
}

func TestWrongMethod(t *testing.T) {
	server := httptest.NewServer(webhook.NewController(&osClient{}, map[string]webhook.Plugin{"github": New()}))
	defer server.Close()

	resp, _ := http.Get(server.URL + "/build100/secret101/github")
	body, _ := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusBadRequest ||
		!strings.Contains(string(body), "method") {
		t.Errorf("Expected BadRequest , got %s: %s!", resp.Status, string(body))
	}
}

func TestWrongContentType(t *testing.T) {
	server := httptest.NewServer(webhook.NewController(&osClient{}, map[string]webhook.Plugin{"github": New()}))
	defer server.Close()

	client := &http.Client{}
	req, _ := http.NewRequest("POST", server.URL+"/build100/secret101/github", nil)
	req.Header.Add("Content-Type", "application/text")
	req.Header.Add("User-Agent", "GitHub-Hookshot/github")
	req.Header.Add("X-Github-Event", "ping")
	resp, _ := client.Do(req)
	body, _ := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusBadRequest ||
		!strings.Contains(string(body), "Content-Type") {
		t.Errorf("Excepcted BadRequest, got %s: %s!", resp.Status, string(body))
	}
}

func TestWrongUserAgent(t *testing.T) {
	server := httptest.NewServer(webhook.NewController(&osClient{}, map[string]webhook.Plugin{"github": New()}))
	defer server.Close()

	client := &http.Client{}
	req, _ := http.NewRequest("POST", server.URL+"/build100/secret101/github", nil)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("User-Agent", "go-lang")
	req.Header.Add("X-Github-Event", "ping")
	resp, _ := client.Do(req)
	body, _ := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusBadRequest ||
		!strings.Contains(string(body), "User-Agent") {
		t.Errorf("Excepcted BadRequest, got %s: %s!", resp.Status, string(body))
	}
}

func TestMissingGithubEvent(t *testing.T) {
	server := httptest.NewServer(webhook.NewController(&osClient{}, map[string]webhook.Plugin{"github": New()}))
	defer server.Close()

	client := &http.Client{}
	req, _ := http.NewRequest("POST", server.URL+"/build100/secret101/github", nil)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("User-Agent", "GitHub-Hookshot/github")
	resp, _ := client.Do(req)
	body, _ := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusBadRequest ||
		!strings.Contains(string(body), "X-GitHub-Event") {
		t.Errorf("Excepcted BadRequest, got %s: %s!", resp.Status, string(body))
	}
}

func TestWrongGithubEvent(t *testing.T) {
	server := httptest.NewServer(webhook.NewController(&osClient{}, map[string]webhook.Plugin{"github": New()}))
	defer server.Close()

	client := &http.Client{}
	req, _ := http.NewRequest("POST", server.URL+"/build100/secret101/github", nil)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("User-Agent", "GitHub-Hookshot/github")
	req.Header.Add("X-GitHub-Event", "wrong")
	resp, _ := client.Do(req)
	body, _ := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusBadRequest ||
		!strings.Contains(string(body), "Unknown") {
		t.Errorf("Excepcted BadRequest, got %s: %s!", resp.Status, string(body))
	}
}

func TestJsonPingEvent(t *testing.T) {
	server := httptest.NewServer(webhook.NewController(&osClient{}, map[string]webhook.Plugin{"github": New()}))
	defer server.Close()

	postFile("ping", "pingevent.json", server.URL+"/build100/secret101/github",
		http.StatusOK, t)
}

func TestJsonPushEventError(t *testing.T) {
	server := httptest.NewServer(webhook.NewController(&osClient{}, map[string]webhook.Plugin{"github": New()}))
	defer server.Close()

	post("push", []byte{}, server.URL+"/build100/secret101/github", http.StatusBadRequest, t)
}

func TestJsonPushEvent(t *testing.T) {
	server := httptest.NewServer(webhook.NewController(&osClient{}, map[string]webhook.Plugin{"github": New()}))
	defer server.Close()

	postFile("push", "pushevent.json", server.URL+"/build100/secret101/github",
		http.StatusOK, t)
}

func postFile(event, filename, url string, expStatusCode int, t *testing.T) {
	data, err := ioutil.ReadFile("fixtures/" + filename)
	if err != nil {
		t.Errorf("Failed to open %s: %v", filename, err)
	}

	post(event, data, url, expStatusCode, t)
}

func post(event string, data []byte, url string, expStatusCode int, t *testing.T) {
	client := &http.Client{}
	req, err := http.NewRequest("POST", url, bytes.NewReader(data))
	if err != nil {
		t.Errorf("Error creating POST request: %v!", err)
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("User-Agent", "GitHub-Hookshot/github")
	req.Header.Add("X-Github-Event", event)
	resp, err := client.Do(req)

	if err != nil {
		t.Errorf("Failed posting webhook to: %s!", url)
	}
	body, _ := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != expStatusCode {
		t.Errorf("Wrong response code, expecting %d, got %s: %s!",
			expStatusCode, resp.Status, string(body))
	}
}

type testContext struct {
	plugin   GitHubWebHook
	buildCfg *api.BuildConfig
	req      *http.Request
	path     string
}

func setup(t *testing.T, filename, eventType string) *testContext {
	context := testContext{
		plugin: GitHubWebHook{},
		buildCfg: &api.BuildConfig{
			Secret: "secret101",
			Parameters: api.BuildParameters{
				Source: api.BuildSource{
					Type: api.BuildSourceGit,
					Git: &api.GitBuildSource{
						URI: "git://github.com/my/repo.git",
					},
				},
			},
		},
		path: "/foobar",
	}
	event, err := ioutil.ReadFile("fixtures/" + filename)
	if err != nil {
		t.Errorf("Failed to open %s: %v", filename, err)
	}
	req, err := http.NewRequest("POST", "http://origin.com", bytes.NewReader(event))
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("User-Agent", "GitHub-Hookshot/github")
	req.Header.Add("X-Github-Event", eventType)

	context.req = req
	return &context
}

func TestExtractForAPingEvent(t *testing.T) {
	//setup
	context := setup(t, "pingevent.json", "ping")

	//execute
	_, proceed, err := context.plugin.Extract(context.buildCfg, context.path, context.req)

	//validation
	if err != nil {
		t.Errorf("Error while extracting build info: %s", err)
	}
	if proceed {
		t.Errorf("The 'proceed' return value should equal 'false' %s", proceed)
	}
}

func TestExtractProvidesValidBuildForAPushEvent(t *testing.T) {
	//setup
	context := setup(t, "pushevent.json", "push")

	//execute
	build, proceed, err := context.plugin.Extract(context.buildCfg, context.path, context.req)

	//validation
	if err != nil {
		t.Errorf("Error while extracting build info: %s", err)
	}
	if !proceed {
		t.Errorf("The 'proceed' return value should equal 'true' %s", proceed)
	}
	if build == nil {
		t.Error("Expecting the build to not be nil")
	} else {
		if build.Parameters.Revision.Git.Commit != "9bdc3a26ff933b32f3e558636b58aea86a69f051" {
			t.Error("Expecting the build's desired input to contain the commit id from the push event")
		}
	}
}

func TestExtractProvidesValidBuildForAPushEventOtherThanMaster(t *testing.T) {
	//setup
	context := setup(t, "pushevent-not-master-branch.json", "push")
	context.buildCfg.Parameters.Source.Git.Ref = "my_other_branch"

	//execute
	build, proceed, err := context.plugin.Extract(context.buildCfg, context.path, context.req)

	//validation
	if err != nil {
		t.Errorf("Error while extracting build info: %s", err)
	}
	if !proceed {
		t.Errorf("The 'proceed' return value should equal 'true' %s", proceed)
	}
	if build == nil {
		t.Error("Expecting the build to not be nil")
	} else {
		if build.Parameters.Revision.Git.Commit != "9bdc3a26ff933b32f3e558636b58aea86a69f051" {
			t.Error("Expecting the build's desired input to contain the commit id from the push event")
		}
	}
}

func TestExtractSkipsBuildForUnmatchedBranches(t *testing.T) {
	//setup
	context := setup(t, "pushevent.json", "push")
	context.buildCfg.Parameters.Source.Git.Ref = "adfj32qrafdavckeaewra"

	//execute
	build, proceed, _ := context.plugin.Extract(context.buildCfg, context.path, context.req)
	if proceed {
		t.Errorf("Expecting to not continue from this event because the branch '%s' is not for this buildConfig '%s'", build.Parameters.Source.Git.Ref, context.buildCfg.Parameters.Source.Git.Ref)
	}
}
