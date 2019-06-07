package gitlab

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"

	buildv1 "github.com/openshift/api/build/v1"
)

var mockBuildStrategy = buildv1.BuildStrategy{
	SourceStrategy: &buildv1.SourceBuildStrategy{
		From: corev1.ObjectReference{
			Kind: "DockerImage",
			Name: "repository/image",
		},
	},
}

var buildConfig = &buildv1.BuildConfig{
	Spec: buildv1.BuildConfigSpec{
		Triggers: []buildv1.BuildTriggerPolicy{
			{
				Type: buildv1.GitLabWebHookBuildTriggerType,
				GitLabWebHook: &buildv1.WebHookTrigger{
					Secret: "secret100",
				},
			},
		},
		CommonSpec: buildv1.CommonSpec{
			Source: buildv1.BuildSource{
				Git: &buildv1.GitBuildSource{},
			},
		},
	},
}

func GivenRequest(method string) *http.Request {
	req, _ := http.NewRequest(method, "http://someurl.com", nil)
	return req
}

func TestVerifyRequestForMethod(t *testing.T) {
	req := GivenRequest("GET")
	plugin := New()
	revision, _, _, proceed, err := plugin.Extract(buildConfig, buildConfig.Spec.Triggers[0].GitLabWebHook, req)

	if err == nil || !strings.Contains(err.Error(), "unsupported HTTP method") {
		t.Errorf("Expected unsupported HTTP method, got %v", err)
	}
	if proceed {
		t.Error("Expected 'proceed' return value to be 'false'")
	}
	if revision != nil {
		t.Error("Expected the 'revision' return value to be nil")
	}
}

func TestMissingEvent(t *testing.T) {
	req := GivenRequest("POST")
	req.Header.Add("Content-Type", "application/json")
	plugin := New()
	revision, _, _, proceed, err := plugin.Extract(buildConfig, buildConfig.Spec.Triggers[0].GitLabWebHook, req)

	if err == nil || !strings.Contains(err.Error(), "missing X-Gitlab-Event") {
		t.Errorf("Expected missing X-Gitlab-Event, got %v", err)
	}
	if proceed {
		t.Error("Expected 'proceed' return value to be 'false'")
	}
	if revision != nil {
		t.Error("Expected the 'revision' return value to be nil")
	}
}

func TestWrongGitLabEvent(t *testing.T) {
	req := GivenRequest("POST")
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("X-Gitlab-Event", "wrong")
	plugin := New()
	revision, _, _, proceed, err := plugin.Extract(buildConfig, buildConfig.Spec.Triggers[0].GitLabWebHook, req)

	if err == nil || !strings.Contains(err.Error(), "Unknown X-Gitlab-Event") {
		t.Errorf("Expected missing Unknown X-Gitlab-Event, got %v", err)
	}
	if proceed {
		t.Error("Expected 'proceed' return value to be 'false'")
	}
	if revision != nil {
		t.Error("Expected the 'revision' return value to be nil")
	}
}

func TestJsonPushEventError(t *testing.T) {
	req := post("X-Gitlab-Event", "Push Hook", []byte{}, "http://some.url", http.StatusBadRequest, t)
	plugin := New()
	revision, _, _, proceed, err := plugin.Extract(buildConfig, buildConfig.Spec.Triggers[0].GitLabWebHook, req)

	if err == nil || !strings.Contains(err.Error(), "unexpected end of JSON input") {
		t.Errorf("Expected unexpected end of JSON input, got %v", err)
	}
	if proceed {
		t.Error("Expected 'proceed' return value to be 'false'")
	}
	if revision != nil {
		t.Error("Expected the 'revision' return value to be nil")
	}
}

func TestJsonGitLabPushEvent(t *testing.T) {
	req := postFile("X-Gitlab-Event", "Push Hook", "pushevent.json", "http://some.url", http.StatusOK, t)
	plugin := New()
	_, _, _, proceed, err := plugin.Extract(buildConfig, buildConfig.Spec.Triggers[0].GitLabWebHook, req)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !proceed {
		t.Error("Expected 'proceed' return value to be 'true'")
	}
}

func TestJsonGitLabPushEventWithCharset(t *testing.T) {
	req := postFileWithCharset("X-Gitlab-Event", "Push Hook", "pushevent.json", "http://some.url", "application/json; charset=utf-8", http.StatusOK, t)
	plugin := New()
	_, _, _, proceed, err := plugin.Extract(buildConfig, buildConfig.Spec.Triggers[0].GitLabWebHook, req)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !proceed {
		t.Error("Expected 'proceed' return value to be 'true'")
	}
}

func postFile(eventHeader, eventName, filename, url string, expStatusCode int, t *testing.T) *http.Request {
	return postFileWithCharset(eventHeader, eventName, filename, url, "application/json", expStatusCode, t)
}

func postFileWithCharset(eventHeader, eventName, filename, url, charset string, expStatusCode int, t *testing.T) *http.Request {
	data, err := ioutil.ReadFile("testdata/" + filename)
	if err != nil {
		t.Errorf("Failed to open %s: %v", filename, err)
	}

	return postWithCharset(eventHeader, eventName, data, url, charset, expStatusCode, t)
}

func post(eventHeader, eventName string, data []byte, url string, expStatusCode int, t *testing.T) *http.Request {
	return postWithCharset(eventHeader, eventName, data, url, "application/json", expStatusCode, t)
}

func postWithCharset(eventHeader, eventName string, data []byte, url, charset string, expStatusCode int, t *testing.T) *http.Request {
	req, err := http.NewRequest("POST", url, bytes.NewReader(data))
	if err != nil {
		t.Errorf("Error creating POST request: %v", err)
	}

	req.Header.Add("Content-Type", charset)
	req.Header.Add(eventHeader, eventName)

	return req
}

type testContext struct {
	plugin   WebHookPlugin
	buildCfg *buildv1.BuildConfig
	req      *http.Request
	path     string
}

func setup(t *testing.T, filename, eventType, ref string) *testContext {
	context := testContext{
		plugin: WebHookPlugin{},
		buildCfg: &buildv1.BuildConfig{
			Spec: buildv1.BuildConfigSpec{
				Triggers: []buildv1.BuildTriggerPolicy{
					{
						Type: buildv1.GitLabWebHookBuildTriggerType,
						GitLabWebHook: &buildv1.WebHookTrigger{
							Secret: "secret101",
						},
					},
					{
						Type: buildv1.GitLabWebHookBuildTriggerType,
						GitLabWebHook: &buildv1.WebHookTrigger{
							Secret: "secret100",
						},
					},
					{
						Type: buildv1.GitLabWebHookBuildTriggerType,
						GitLabWebHook: &buildv1.WebHookTrigger{
							Secret: "secret102",
						},
					},
				},
				CommonSpec: buildv1.CommonSpec{
					Source: buildv1.BuildSource{
						Git: &buildv1.GitBuildSource{
							URI: "git://github.com/my/repo.git",
							Ref: ref,
						},
					},
					Strategy: mockBuildStrategy,
				},
			},
		},
		path: "/foobar",
	}
	event, err := ioutil.ReadFile("testdata/" + filename)
	if err != nil {
		t.Errorf("Failed to open %s: %v", filename, err)
	}
	req, err := http.NewRequest("POST", "http://origin.com", bytes.NewReader(event))
	if err != nil {
		t.Errorf("Failed to create a new request (%s)", err)
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("X-Gitlab-Event", eventType)

	context.req = req
	return &context
}

func TestExtractProvidesValidBuildForAPushEvent(t *testing.T) {
	//setup
	context := setup(t, "pushevent.json", "Push Hook", "")

	//execute
	revision, _, _, proceed, err := context.plugin.Extract(context.buildCfg, buildConfig.Spec.Triggers[0].GitLabWebHook, context.req)

	//validation
	if err != nil {
		t.Errorf("Error while extracting build info: %s", err)
	}
	if !proceed {
		t.Errorf("The 'proceed' return value should equal 'true' %t", proceed)
	}
	if revision == nil {
		t.Fatal("Expecting the revision to not be nil")
	}
	if revision.Git.Commit != "da1560886d4f094c3e6c9ef40349f7d38b5d27d7" {
		t.Errorf("Expecting the revision to contain the commit id from the push event, got %#v", revision.Git.Commit)
	}

}

func TestExtractProvidesValidBuildForAPushEventOtherThanMaster(t *testing.T) {
	//setup
	context := setup(t, "pushevent-not-master-branch.json", "Push Hook", "my_other_branch")
	//execute
	revision, _, _, proceed, err := context.plugin.Extract(context.buildCfg, buildConfig.Spec.Triggers[0].GitLabWebHook, context.req)

	//validation
	if err != nil {
		t.Errorf("Error while extracting build info: %s", err)
	}
	if !proceed {
		t.Errorf("The 'proceed' return value should equal 'true' %t", proceed)
	}
	if revision == nil {
		t.Fatal("Expecting the revision to not be nil")
	}
	if revision.Git.Commit != "da1560886d4f094c3e6c9ef40349f7d38b5d27d7" {
		t.Error("Expecting the revision to contain the commit id from the push event")
	}
}

func TestExtractSkipsBuildForUnmatchedBranches(t *testing.T) {
	//setup
	context := setup(t, "pushevent.json", "Push Hook", "wrongref")

	//execute
	_, _, _, proceed, err := context.plugin.Extract(context.buildCfg, buildConfig.Spec.Triggers[0].GitLabWebHook, context.req)
	if err != nil {
		t.Errorf("Error while extracting build info: %s", err)
	}
	if proceed {
		t.Errorf("Expecting to not continue from this event because the branch is not for this buildConfig '%s'", context.buildCfg.Spec.Source.Git.Ref)
	}
}
