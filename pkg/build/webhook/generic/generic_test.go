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
)

var mockBuildStrategy api.BuildStrategy = api.BuildStrategy{
	Type: "STI",
	STIStrategy: &api.STIBuildStrategy{
		From: &kapi.ObjectReference{
			Name: "repository/image",
		},
	},
}

func GivenRequest(method string) *http.Request {
	req, _ := http.NewRequest(method, "http://someurl.com", nil)
	return req
}

func GivenRequestWithPayload(t *testing.T) *http.Request {
	data, err := ioutil.ReadFile("fixtures/push-git.json")
	if err != nil {
		t.Errorf("Error reading setup data: %v", err)
		return nil
	}
	req, _ := http.NewRequest("POST", "http://someurl.com", bytes.NewReader(data))
	req.Header.Add("User-Agent", "Some User Agent")
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
	req.Header.Add("User-Agent", "Some User Agent")
	req.Header.Add("Content-Type", "application/json")
	return req
}

func TestVerifyRequestForMethod(t *testing.T) {
	req := GivenRequest("GET")
	err := verifyRequest(req)
	if err == nil || !strings.Contains(err.Error(), "method") {
		t.Errorf("Expected anything but POST to be an invalid method %v", err)
	}
}

func TestVerifyRequestForUserAgent(t *testing.T) {
	req := &http.Request{
		Header: http.Header{"Content-Type": {"application/json"}},
		Method: "POST",
	}
	err := verifyRequest(req)
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}

	req.Header.Add("User-Agent", "")
	err = verifyRequest(req)
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}

	req.Header.Set("User-Agent", "foobar")
	err = verifyRequest(req)
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}

}

func TestVerifyRequestForContentType(t *testing.T) {
	req := &http.Request{
		Header: http.Header{"User-Agent": {"foobar"}},
		Method: "POST",
	}
	err := verifyRequest(req)
	if err != nil && !strings.Contains(err.Error(), "Content-Type") {
		t.Errorf("Exp. a content type error")
	}

	req.Header.Add("Content-Length", "1")
	err = verifyRequest(req)
	if err == nil || !strings.Contains(err.Error(), "Content-Type") {
		t.Errorf("Exp. ContentType to be required if a payload is posted %v", err)
	}

	req.Header.Add("Content-Type", "X-Whatever")
	err = verifyRequest(req)
	if err == nil || !strings.Contains(err.Error(), "Unsupported Content-Type") {
		t.Errorf("Exp. to only support json payloads %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	err = verifyRequest(req)
	if err != nil && !strings.Contains(err.Error(), "Unsupported Content-Type") {
		t.Errorf("Exp. to allow json payloads %v", err)
	}
}

func TestExtractWithEmptyPayload(t *testing.T) {
	req := GivenRequest("POST")
	req.Header.Add("User-Agent", "Some User Agent")
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
	req := GivenRequestWithPayload(t)
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
		t.Error("Expected the 'revision' return value to be nil since we arent creating a new one")
	}
}

func TestExtractWithGitPayload(t *testing.T) {
	req := GivenRequestWithPayload(t)
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

type errJSON struct{}

func (*errJSON) Read(p []byte) (n int, err error) {
	p = []byte("{")
	return len(p), io.EOF
}

func TestExtractWithUnmarshalError(t *testing.T) {
	req, _ := http.NewRequest("POST", "http://someurl.com", &errJSON{})
	req.Header.Add("User-Agent", "Some User Agent")
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
