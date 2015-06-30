package generic

import (
	"bytes"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/build/webhook"
)

var mockBuildStrategy = api.BuildStrategy{
	Type: "STI",
	SourceStrategy: &api.SourceBuildStrategy{
		From: kapi.ObjectReference{
			Name: "repository/image",
		},
	},
}

func GivenRequest(method string) *http.Request {
	req, _ := http.NewRequest(method, "http://someurl.com", nil)
	return req
}

func GivenRequestWithPayload(t *testing.T, filename string) *http.Request {
	data, err := ioutil.ReadFile("fixtures/" + filename)
	if err != nil {
		t.Errorf("Error reading setup data: %v", err)
		return nil
	}
	req, _ := http.NewRequest("POST", "http://someurl.com", bytes.NewReader(data))
	req.Header.Add("Content-Type", "application/json")
	return req
}

func GivenRequestWithRefsPayload(t *testing.T) *http.Request {
	data, err := ioutil.ReadFile("fixtures/post-receive-git.json")
	if err != nil {
		t.Errorf("Error reading setup data: %v", err)
		return nil
	}
	req, _ := http.NewRequest("POST", "http://someurl.com", bytes.NewReader(data))
	req.Header.Add("Content-Type", "application/json")
	return req
}

func TestVerifyRequestForMethod(t *testing.T) {
	req := GivenRequest("GET")
	buildConfig := &api.BuildConfig{
		Triggers: []api.BuildTriggerPolicy{
			{
				Type: api.GenericWebHookBuildTriggerType,
				GenericWebHook: &api.WebHookTrigger{
					Secret: "secret100",
				},
			},
		},
	}
	plugin := New()
	revision, proceed, err := plugin.Extract(buildConfig, "secret100", "", req)

	if err == nil || !strings.Contains(err.Error(), "Unsupported HTTP method") {
		t.Errorf("Excepcted unsupported HTTP method, got %v!", err)
	}
	if proceed {
		t.Error("Expected 'proceed' return value to be 'false'")
	}
	if revision != nil {
		t.Error("Expected the 'revision' return value to be nil")
	}
}

func TestWrongSecret(t *testing.T) {
	req := GivenRequest("POST")
	buildConfig := &api.BuildConfig{
		Triggers: []api.BuildTriggerPolicy{
			{
				Type: api.GenericWebHookBuildTriggerType,
				GenericWebHook: &api.WebHookTrigger{
					Secret: "secret100",
				},
			},
		},
	}
	plugin := New()
	revision, proceed, err := plugin.Extract(buildConfig, "wrongsecret", "", req)

	if err != webhook.ErrSecretMismatch {
		t.Errorf("Excepcted %v, got %v!", webhook.ErrSecretMismatch, err)
	}
	if proceed {
		t.Error("Expected 'proceed' return value to be 'false'")
	}
	if revision != nil {
		t.Error("Expected the 'revision' return value to be nil")
	}
}

type emptyReader struct{}

func (_ emptyReader) Read(p []byte) (n int, err error) {
	return 0, io.EOF
}

func TestExtractWithEmptyPayload(t *testing.T) {
	req, _ := http.NewRequest("POST", "http://someurl.com", emptyReader{})
	req.Header.Add("Content-Type", "application/json")
	buildConfig := &api.BuildConfig{
		Triggers: []api.BuildTriggerPolicy{
			{
				Type: api.GenericWebHookBuildTriggerType,
				GenericWebHook: &api.WebHookTrigger{
					Secret: "secret100",
				},
			},
		},
		Parameters: api.BuildParameters{
			Source: api.BuildSource{
				Type: api.BuildSourceGit,
				Git: &api.GitBuildSource{
					Ref: "master",
				},
			},
			Strategy: mockBuildStrategy,
		},
	}
	plugin := New()
	revision, proceed, err := plugin.Extract(buildConfig, "secret100", "", req)
	if err != nil {
		t.Errorf("Expected to be able to trigger a build without a payload error: %v", err)
	}
	if !proceed {
		t.Error("Expected 'proceed' return value to be 'true'")
	}
	if revision != nil {
		t.Error("Expected the 'revision' return value to be nil")
	}
}

func TestExtractWithUnmatchedRefGitPayload(t *testing.T) {
	req := GivenRequestWithPayload(t, "push-github.json")
	buildConfig := &api.BuildConfig{
		Triggers: []api.BuildTriggerPolicy{
			{
				Type: api.GenericWebHookBuildTriggerType,
				GenericWebHook: &api.WebHookTrigger{
					Secret: "secret100",
				},
			},
		},
		Parameters: api.BuildParameters{
			Source: api.BuildSource{
				Type: api.BuildSourceGit,
				Git: &api.GitBuildSource{
					Ref: "asdfkasdfasdfasdfadsfkjhkhkh",
				},
			},
			Strategy: mockBuildStrategy,
		},
	}
	plugin := New()
	build, proceed, err := plugin.Extract(buildConfig, "secret100", "", req)

	if err != nil {
		t.Errorf("Unexpected error when triggering build: %v", err)
	}
	if proceed {
		t.Error("Expected 'proceed' return value to be 'false' for unmatched refs")
	}
	if build != nil {
		t.Error("Expected the 'revision' return value to be nil since we aren't creating a new one")
	}
}

func TestExtractWithGitPayload(t *testing.T) {
	req := GivenRequestWithPayload(t, "push-github.json")
	buildConfig := &api.BuildConfig{
		Triggers: []api.BuildTriggerPolicy{
			{
				Type: api.GenericWebHookBuildTriggerType,
				GenericWebHook: &api.WebHookTrigger{
					Secret: "secret100",
				},
			},
		},
		Parameters: api.BuildParameters{
			Source: api.BuildSource{
				Type: api.BuildSourceGit,
				Git: &api.GitBuildSource{
					Ref: "master",
				},
			},
			Strategy: mockBuildStrategy,
		},
	}
	plugin := New()
	revision, proceed, err := plugin.Extract(buildConfig, "secret100", "", req)

	if err != nil {
		t.Errorf("Expected to be able to trigger a build without a payload error: %v", err)
	}
	if !proceed {
		t.Error("Expected 'proceed' return value to be 'true'")
	}
	if revision == nil {
		t.Error("Expected the 'revision' return value to not be nil")
	}
}

func TestExtractWithGitRefsPayload(t *testing.T) {
	req := GivenRequestWithRefsPayload(t)
	buildConfig := &api.BuildConfig{
		Triggers: []api.BuildTriggerPolicy{
			{
				Type: api.GenericWebHookBuildTriggerType,
				GenericWebHook: &api.WebHookTrigger{
					Secret: "secret100",
				},
			},
		},
		Parameters: api.BuildParameters{
			Source: api.BuildSource{
				Type: api.BuildSourceGit,
				Git: &api.GitBuildSource{
					Ref: "master",
				},
			},
			Strategy: mockBuildStrategy,
		},
	}
	plugin := New()
	revision, proceed, err := plugin.Extract(buildConfig, "secret100", "", req)

	if err != nil {
		t.Errorf("Expected to be able to trigger a build without a payload error: %v", err)
	}
	if !proceed {
		t.Error("Expected 'proceed' return value to be 'true'")
	}
	if revision == nil {
		t.Error("Expected the 'revision' return value to not be nil")
	}
}

func TestExtractWithUnmatchedGitRefsPayload(t *testing.T) {
	req := GivenRequestWithRefsPayload(t)
	buildConfig := &api.BuildConfig{
		Triggers: []api.BuildTriggerPolicy{
			{
				Type: api.GenericWebHookBuildTriggerType,
				GenericWebHook: &api.WebHookTrigger{
					Secret: "secret100",
				},
			},
		},
		Parameters: api.BuildParameters{
			Source: api.BuildSource{
				Type: api.BuildSourceGit,
				Git: &api.GitBuildSource{
					Ref: "other",
				},
			},
			Strategy: mockBuildStrategy,
		},
	}
	plugin := New()
	revision, proceed, err := plugin.Extract(buildConfig, "secret100", "", req)

	if err != nil {
		t.Errorf("Expected to be able to trigger a build without a payload error: %v", err)
	}
	if proceed {
		t.Error("Expected 'proceed' return value to be 'false'")
	}
	if revision != nil {
		t.Error("Expected the 'revision' return value to be nil")
	}
}

func TestGitlabPush(t *testing.T) {
	req := GivenRequestWithPayload(t, "push-gitlab.json")
	buildConfig := &api.BuildConfig{
		Triggers: []api.BuildTriggerPolicy{
			{
				Type: api.GenericWebHookBuildTriggerType,
				GenericWebHook: &api.WebHookTrigger{
					Secret: "secret100",
				},
			},
		},
		Parameters: api.BuildParameters{
			Source: api.BuildSource{
				Type: api.BuildSourceGit,
			},
			Strategy: mockBuildStrategy,
		},
	}
	plugin := New()
	_, proceed, err := plugin.Extract(buildConfig, "secret100", "", req)

	if err != nil {
		t.Errorf("Expected to be able to trigger a build without a payload error: %v", err)
	}
	if !proceed {
		t.Error("Expected 'proceed' return value to be 'true'")
	}
}
func TestNonJsonPush(t *testing.T) {
	req, _ := http.NewRequest("POST", "http://someurl.com", nil)
	req.Header.Add("Content-Type", "*/*")

	buildConfig := &api.BuildConfig{
		Triggers: []api.BuildTriggerPolicy{
			{
				Type: api.GenericWebHookBuildTriggerType,
				GenericWebHook: &api.WebHookTrigger{
					Secret: "secret100",
				},
			},
		},
		Parameters: api.BuildParameters{
			Source: api.BuildSource{
				Type: api.BuildSourceGit,
			},
			Strategy: mockBuildStrategy,
		},
	}
	plugin := New()
	_, proceed, err := plugin.Extract(buildConfig, "secret100", "", req)

	if err != nil {
		t.Errorf("Expected to be able to trigger a build without a payload error: %v", err)
	}
	if !proceed {
		t.Error("Expected 'proceed' return value to be 'true'")
	}
}

type errJSON struct{}

func (_ errJSON) Read(p []byte) (n int, err error) {
	p = []byte("{")
	return len(p), io.EOF
}

func TestExtractWithUnmarshalError(t *testing.T) {
	req, _ := http.NewRequest("POST", "http://someurl.com", errJSON{})
	req.Header.Add("Content-Type", "application/json")
	buildConfig := &api.BuildConfig{
		Triggers: []api.BuildTriggerPolicy{
			{
				Type: api.GenericWebHookBuildTriggerType,
				GenericWebHook: &api.WebHookTrigger{
					Secret: "secret100",
				},
			},
		},
		Parameters: api.BuildParameters{
			Source: api.BuildSource{
				Type: api.BuildSourceGit,
				Git: &api.GitBuildSource{
					Ref: "other",
				},
			},
			Strategy: mockBuildStrategy,
		},
	}
	plugin := New()
	revision, proceed, err := plugin.Extract(buildConfig, "secret100", "", req)
	if err != nil {
		t.Errorf("Expected to be able to trigger a build without a payload error: %v", err)
	}
	if !proceed {
		t.Error("Expected 'proceed' return value to be 'true'")
	}
	if revision != nil {
		t.Error("Expected the 'revision' return value to be nil")
	}
}
