package generic

import (
	"bytes"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/build/webhook"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/unversioned"
)

var mockBuildStrategy = api.BuildStrategy{
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
	return GivenRequestWithPayloadAndContentType(t, filename, "application/json")
}

func GivenRequestWithPayloadAndContentType(t *testing.T, filename, contentType string) *http.Request {
	data, err := ioutil.ReadFile("testdata/" + filename)
	if err != nil {
		t.Errorf("Error reading setup data: %v", err)
		return nil
	}
	req, _ := http.NewRequest("POST", "http://someurl.com", bytes.NewReader(data))
	req.Header.Add("Content-Type", contentType)
	return req
}

func GivenRequestWithRefsPayload(t *testing.T) *http.Request {
	data, err := ioutil.ReadFile("testdata/post-receive-git.json")
	if err != nil {
		t.Errorf("Error reading setup data: %v", err)
		return nil
	}
	req, _ := http.NewRequest("POST", "http://someurl.com", bytes.NewReader(data))
	req.Header.Add("Content-Type", "application/json")
	return req
}

func matchWarning(t *testing.T, err error, message string) {
	status, ok := err.(*errors.StatusError)
	if !ok {
		t.Errorf("Expected %v to be a StatusError object", err)
		return
	}

	if status.ErrStatus.Status != unversioned.StatusSuccess {
		t.Errorf("Unexpected response status %v, expected %v", status.ErrStatus.Status, unversioned.StatusSuccess)
	}
	if status.ErrStatus.Code != http.StatusOK {
		t.Errorf("Unexpected response code %v, expected %v", status.ErrStatus.Code, http.StatusOK)
	}
	if status.ErrStatus.Message != message {
		t.Errorf("Unexpected response message %v, expected %v", status.ErrStatus.Message, message)
	}
}

func TestVerifyRequestForMethod(t *testing.T) {
	req := GivenRequest("GET")
	buildConfig := &api.BuildConfig{
		Spec: api.BuildConfigSpec{
			Triggers: []api.BuildTriggerPolicy{
				{
					Type: api.GenericWebHookBuildTriggerType,
					GenericWebHook: &api.WebHookTrigger{
						Secret: "secret100",
					},
				},
			},
		},
	}
	plugin := New()
	revision, _, proceed, err := plugin.Extract(buildConfig, "secret100", "", req)

	if err == nil || !strings.Contains(err.Error(), "unsupported HTTP method") {
		t.Errorf("Expected unsupported HTTP method, got %v!", err)
	}
	if proceed {
		t.Error("Expected 'proceed' return value to be 'false'")
	}
	if revision != nil {
		t.Error("Expected the 'revision' return value to be nil")
	}
}

func TestWrongSecretMultipleGenericWebHooks(t *testing.T) {
	req := GivenRequest("GET")
	buildConfig := &api.BuildConfig{
		Spec: api.BuildConfigSpec{
			Triggers: []api.BuildTriggerPolicy{
				{
					Type: api.GenericWebHookBuildTriggerType,
					GenericWebHook: &api.WebHookTrigger{
						Secret: "secret101",
					},
				},
				{
					Type: api.GenericWebHookBuildTriggerType,
					GenericWebHook: &api.WebHookTrigger{
						Secret: "secret100",
					},
				},
				{
					Type: api.GenericWebHookBuildTriggerType,
					GenericWebHook: &api.WebHookTrigger{
						Secret: "secret102",
					},
				},
			},
		},
	}

	plugin := New()
	revision, _, proceed, err := plugin.Extract(buildConfig, "wrongsecret", "", req)

	if err != webhook.ErrSecretMismatch {
		t.Errorf("Expected %s, got %s", webhook.ErrSecretMismatch, err)
	}

	if proceed {
		t.Error("Expected 'proceed' to return 'false'")
	}

	if revision != nil {
		t.Errorf("Expected the 'revision' to be nil, go %v instead", revision)
	}
}

func TestMatchSecretMultipleGenericWebHooks(t *testing.T) {
	req := GivenRequestWithPayload(t, "push-generic.json")
	buildConfig := &api.BuildConfig{
		Spec: api.BuildConfigSpec{
			Triggers: []api.BuildTriggerPolicy{
				{
					Type: api.GenericWebHookBuildTriggerType,
					GenericWebHook: &api.WebHookTrigger{
						Secret: "secret101",
					},
				},
				{
					Type: api.GenericWebHookBuildTriggerType,
					GenericWebHook: &api.WebHookTrigger{
						Secret: "secret100",
					},
				},
				{
					Type: api.GenericWebHookBuildTriggerType,
					GenericWebHook: &api.WebHookTrigger{
						Secret: "secret102",
					},
				},
			},
			CommonSpec: api.CommonSpec{
				Source: api.BuildSource{
					Git: &api.GitBuildSource{
						Ref: "master",
					},
				},
				Strategy: mockBuildStrategy,
			},
		},
	}

	plugin := New()
	revision, _, proceed, err := plugin.Extract(buildConfig, "secret102", "", req)

	if err != nil {
		t.Errorf("Expected no error, got %s", err)
	}

	if !proceed {
		t.Error("Expected 'proceed' to return 'true', got 'false' instead")
	}

	if revision == nil {
		t.Errorf("Expected the 'revision' to not be nil")
	}
}

func TestEnvVarsMultipleGenericWebHooks(t *testing.T) {
	req := GivenRequestWithPayload(t, "push-generic-envs.json")
	buildConfig := &api.BuildConfig{
		Spec: api.BuildConfigSpec{
			Triggers: []api.BuildTriggerPolicy{
				{
					Type: api.GenericWebHookBuildTriggerType,
					GenericWebHook: &api.WebHookTrigger{
						Secret:   "secret101",
						AllowEnv: true,
					},
				},
				{
					Type: api.GenericWebHookBuildTriggerType,
					GenericWebHook: &api.WebHookTrigger{
						Secret: "secret100",
					},
				},
				{
					Type: api.GenericWebHookBuildTriggerType,
					GenericWebHook: &api.WebHookTrigger{
						Secret: "secret102",
					},
				},
			},
			CommonSpec: api.CommonSpec{
				Source: api.BuildSource{
					Git: &api.GitBuildSource{
						Ref: "master",
					},
				},
				Strategy: mockBuildStrategy,
			},
		},
	}

	plugin := New()
	revision, envvars, proceed, err := plugin.Extract(buildConfig, "secret101", "", req)

	if err != nil {
		t.Errorf("Expected to be able to trigger a build without a payload error: %v", err)
	}

	if !proceed {
		t.Error("Expected 'proceed' to return 'true'")
	}

	if revision == nil {
		t.Errorf("Expected the 'revision' to not be nil")
	}

	if len(envvars) == 0 {
		t.Error("Expected env vars to be set")
	}

}

func TestWrongSecret(t *testing.T) {
	req := GivenRequest("POST")
	buildConfig := &api.BuildConfig{
		Spec: api.BuildConfigSpec{
			Triggers: []api.BuildTriggerPolicy{
				{
					Type: api.GenericWebHookBuildTriggerType,
					GenericWebHook: &api.WebHookTrigger{
						Secret: "secret100",
					},
				},
			},
		},
	}
	plugin := New()
	revision, _, proceed, err := plugin.Extract(buildConfig, "wrongsecret", "", req)

	if err != webhook.ErrSecretMismatch {
		t.Errorf("Expected %v, got %v!", webhook.ErrSecretMismatch, err)
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
		Spec: api.BuildConfigSpec{

			Triggers: []api.BuildTriggerPolicy{
				{
					Type: api.GenericWebHookBuildTriggerType,
					GenericWebHook: &api.WebHookTrigger{
						Secret: "secret100",
					},
				},
			},
			CommonSpec: api.CommonSpec{
				Source: api.BuildSource{
					Git: &api.GitBuildSource{
						Ref: "master",
					},
				},
				Strategy: mockBuildStrategy,
			},
		},
	}
	plugin := New()
	revision, _, proceed, err := plugin.Extract(buildConfig, "secret100", "", req)
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
	req := GivenRequestWithPayload(t, "push-generic.json")
	buildConfig := &api.BuildConfig{
		Spec: api.BuildConfigSpec{
			Triggers: []api.BuildTriggerPolicy{
				{
					Type: api.GenericWebHookBuildTriggerType,
					GenericWebHook: &api.WebHookTrigger{
						Secret: "secret100",
					},
				},
			},
			CommonSpec: api.CommonSpec{
				Source: api.BuildSource{
					Git: &api.GitBuildSource{
						Ref: "asdfkasdfasdfasdfadsfkjhkhkh",
					},
				},
				Strategy: mockBuildStrategy,
			},
		},
	}
	plugin := New()
	build, _, proceed, err := plugin.Extract(buildConfig, "secret100", "", req)

	matchWarning(t, err, `skipping build. Branch reference from "refs/heads/master" does not match configuration`)
	if proceed {
		t.Error("Expected 'proceed' return value to be 'false' for unmatched refs")
	}
	if build != nil {
		t.Error("Expected the 'revision' return value to be nil since we aren't creating a new one")
	}
}

func TestExtractWithGitPayload(t *testing.T) {
	req := GivenRequestWithPayload(t, "push-generic.json")
	buildConfig := &api.BuildConfig{
		Spec: api.BuildConfigSpec{
			Triggers: []api.BuildTriggerPolicy{
				{
					Type: api.GenericWebHookBuildTriggerType,
					GenericWebHook: &api.WebHookTrigger{
						Secret: "secret100",
					},
				},
			},
			CommonSpec: api.CommonSpec{
				Source: api.BuildSource{
					Git: &api.GitBuildSource{
						Ref: "master",
					},
				},
				Strategy: mockBuildStrategy,
			},
		},
	}
	plugin := New()
	revision, _, proceed, err := plugin.Extract(buildConfig, "secret100", "", req)

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

func TestExtractWithGitPayloadAndUTF8Charset(t *testing.T) {
	req := GivenRequestWithPayloadAndContentType(t, "push-generic.json", "application/json; charset=utf-8")
	buildConfig := &api.BuildConfig{
		Spec: api.BuildConfigSpec{
			Triggers: []api.BuildTriggerPolicy{
				{
					Type: api.GenericWebHookBuildTriggerType,
					GenericWebHook: &api.WebHookTrigger{
						Secret: "secret100",
					},
				},
			},
			CommonSpec: api.CommonSpec{
				Source: api.BuildSource{
					Git: &api.GitBuildSource{
						Ref: "master",
					},
				},
				Strategy: mockBuildStrategy,
			},
		},
	}
	plugin := New()
	revision, _, proceed, err := plugin.Extract(buildConfig, "secret100", "", req)

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
		Spec: api.BuildConfigSpec{
			Triggers: []api.BuildTriggerPolicy{
				{
					Type: api.GenericWebHookBuildTriggerType,
					GenericWebHook: &api.WebHookTrigger{
						Secret: "secret100",
					},
				},
			},
			CommonSpec: api.CommonSpec{
				Source: api.BuildSource{
					Git: &api.GitBuildSource{
						Ref: "master",
					},
				},
				Strategy: mockBuildStrategy,
			},
		},
	}
	plugin := New()
	revision, _, proceed, err := plugin.Extract(buildConfig, "secret100", "", req)

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
		Spec: api.BuildConfigSpec{
			Triggers: []api.BuildTriggerPolicy{
				{
					Type: api.GenericWebHookBuildTriggerType,
					GenericWebHook: &api.WebHookTrigger{
						Secret: "secret100",
					},
				},
			},
			CommonSpec: api.CommonSpec{
				Source: api.BuildSource{
					Git: &api.GitBuildSource{
						Ref: "other",
					},
				},
				Strategy: mockBuildStrategy,
			},
		},
	}
	plugin := New()
	revision, _, proceed, err := plugin.Extract(buildConfig, "secret100", "", req)

	matchWarning(t, err, `skipping build. None of the supplied refs matched "other"`)
	if proceed {
		t.Error("Expected 'proceed' return value to be 'false'")
	}
	if revision != nil {
		t.Error("Expected the 'revision' return value to be nil")
	}
}

func TestExtractWithKeyValuePairsJSON(t *testing.T) {
	req := GivenRequestWithPayload(t, "push-generic-envs.json")
	buildConfig := &api.BuildConfig{
		Spec: api.BuildConfigSpec{
			Triggers: []api.BuildTriggerPolicy{
				{
					Type: api.GenericWebHookBuildTriggerType,
					GenericWebHook: &api.WebHookTrigger{
						Secret:   "secret100",
						AllowEnv: true,
					},
				},
			},
			CommonSpec: api.CommonSpec{
				Source: api.BuildSource{
					Git: &api.GitBuildSource{
						Ref: "master",
					},
				},
				Strategy: mockBuildStrategy,
			},
		},
	}
	plugin := New()
	revision, envvars, proceed, err := plugin.Extract(buildConfig, "secret100", "", req)

	if err != nil {
		t.Errorf("Expected to be able to trigger a build without a payload error: %v", err)
	}
	if !proceed {
		t.Error("Expected 'proceed' return value to be 'true'")
	}
	if revision == nil {
		t.Error("Expected the 'revision' return value to not be nil")
	}

	if len(envvars) == 0 {
		t.Error("Expected env vars to be set")
	}
}

func TestExtractWithKeyValuePairsYAML(t *testing.T) {
	req := GivenRequestWithPayloadAndContentType(t, "push-generic-envs.yaml", "application/yaml")
	buildConfig := &api.BuildConfig{
		Spec: api.BuildConfigSpec{
			Triggers: []api.BuildTriggerPolicy{
				{
					Type: api.GenericWebHookBuildTriggerType,
					GenericWebHook: &api.WebHookTrigger{
						Secret:   "secret100",
						AllowEnv: true,
					},
				},
			},
			CommonSpec: api.CommonSpec{
				Source: api.BuildSource{
					Git: &api.GitBuildSource{
						Ref: "master",
					},
				},
				Strategy: mockBuildStrategy,
			},
		},
	}
	plugin := New()
	revision, envvars, proceed, err := plugin.Extract(buildConfig, "secret100", "", req)

	if err != nil {
		t.Errorf("Expected to be able to trigger a build without a payload error: %v", err)
	}
	if !proceed {
		t.Error("Expected 'proceed' return value to be 'true'")
	}
	if revision == nil {
		t.Error("Expected the 'revision' return value to not be nil")
	}

	if len(envvars) == 0 {
		t.Error("Expected env vars to be set")
	}
}

func TestExtractWithKeyValuePairsDisabled(t *testing.T) {
	req := GivenRequestWithPayload(t, "push-generic-envs.json")
	buildConfig := &api.BuildConfig{
		Spec: api.BuildConfigSpec{
			Triggers: []api.BuildTriggerPolicy{
				{
					Type: api.GenericWebHookBuildTriggerType,
					GenericWebHook: &api.WebHookTrigger{
						Secret: "secret100",
					},
				},
			},
			CommonSpec: api.CommonSpec{
				Source: api.BuildSource{
					Git: &api.GitBuildSource{
						Ref: "master",
					},
				},
				Strategy: mockBuildStrategy,
			},
		},
	}
	plugin := New()
	revision, envvars, proceed, err := plugin.Extract(buildConfig, "secret100", "", req)

	if err != nil {
		t.Errorf("Expected to be able to trigger a build without a payload error: %v", err)
	}
	if !proceed {
		t.Error("Expected 'proceed' return value to be 'true'")
	}
	if revision == nil {
		t.Error("Expected the 'revision' return value to not be nil")
	}

	if len(envvars) != 0 {
		t.Error("Expected env vars to be empty")
	}
}

func TestGitlabPush(t *testing.T) {
	req := GivenRequestWithPayload(t, "push-gitlab.json")
	buildConfig := &api.BuildConfig{
		Spec: api.BuildConfigSpec{
			Triggers: []api.BuildTriggerPolicy{
				{
					Type: api.GenericWebHookBuildTriggerType,
					GenericWebHook: &api.WebHookTrigger{
						Secret: "secret100",
					},
				},
			},
			CommonSpec: api.CommonSpec{
				Source: api.BuildSource{
					Git: &api.GitBuildSource{},
				},
				Strategy: mockBuildStrategy,
			},
		},
	}
	plugin := New()
	_, _, proceed, err := plugin.Extract(buildConfig, "secret100", "", req)

	matchWarning(t, err, "no git information found in payload, ignoring and continuing with build")
	if !proceed {
		t.Error("Expected 'proceed' return value to be 'true'")
	}
}
func TestNonJsonPush(t *testing.T) {
	req, _ := http.NewRequest("POST", "http://someurl.com", nil)
	req.Header.Add("Content-Type", "*/*")

	buildConfig := &api.BuildConfig{
		Spec: api.BuildConfigSpec{
			Triggers: []api.BuildTriggerPolicy{
				{
					Type: api.GenericWebHookBuildTriggerType,
					GenericWebHook: &api.WebHookTrigger{
						Secret: "secret100",
					},
				},
			},
			CommonSpec: api.CommonSpec{
				Source: api.BuildSource{
					Git: &api.GitBuildSource{},
				},
				Strategy: mockBuildStrategy,
			},
		},
	}
	plugin := New()
	_, _, proceed, err := plugin.Extract(buildConfig, "secret100", "", req)

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
		Spec: api.BuildConfigSpec{
			Triggers: []api.BuildTriggerPolicy{
				{
					Type: api.GenericWebHookBuildTriggerType,
					GenericWebHook: &api.WebHookTrigger{
						Secret: "secret100",
					},
				},
			},
			CommonSpec: api.CommonSpec{
				Source: api.BuildSource{
					Git: &api.GitBuildSource{
						Ref: "other",
					},
				},
				Strategy: mockBuildStrategy,
			},
		},
	}
	plugin := New()
	revision, _, proceed, err := plugin.Extract(buildConfig, "secret100", "", req)
	matchWarning(t, err, `error unmarshalling payload: invalid character '\x00' looking for beginning of value, ignoring payload and continuing with build`)
	if !proceed {
		t.Error("Expected 'proceed' return value to be 'true'")
	}
	if revision != nil {
		t.Error("Expected the 'revision' return value to be nil")
	}
}

func TestExtractWithUnmarshalErrorYAML(t *testing.T) {
	req, _ := http.NewRequest("POST", "http://someurl.com", errJSON{})
	req.Header.Add("Content-Type", "application/yaml")
	buildConfig := &api.BuildConfig{
		Spec: api.BuildConfigSpec{
			Triggers: []api.BuildTriggerPolicy{
				{
					Type: api.GenericWebHookBuildTriggerType,
					GenericWebHook: &api.WebHookTrigger{
						Secret: "secret100",
					},
				},
			},
			CommonSpec: api.CommonSpec{
				Source: api.BuildSource{
					Git: &api.GitBuildSource{
						Ref: "other",
					},
				},
				Strategy: mockBuildStrategy,
			},
		},
	}
	plugin := New()
	revision, _, proceed, err := plugin.Extract(buildConfig, "secret100", "", req)
	matchWarning(t, err, "error converting payload to json: yaml: control characters are not allowed, ignoring payload and continuing with build")
	if !proceed {
		t.Error("Expected 'proceed' return value to be 'true'")
	}
	if revision != nil {
		t.Error("Expected the 'revision' return value to be nil")
	}
}

func TestExtractWithBadContentType(t *testing.T) {
	req, _ := http.NewRequest("POST", "http://someurl.com", errJSON{})
	req.Header.Add("Content-Type", "bad")
	buildConfig := &api.BuildConfig{
		Spec: api.BuildConfigSpec{
			Triggers: []api.BuildTriggerPolicy{
				{
					Type: api.GenericWebHookBuildTriggerType,
					GenericWebHook: &api.WebHookTrigger{
						Secret: "secret100",
					},
				},
			},
			CommonSpec: api.CommonSpec{
				Source: api.BuildSource{
					Git: &api.GitBuildSource{
						Ref: "other",
					},
				},
				Strategy: mockBuildStrategy,
			},
		},
	}
	plugin := New()
	revision, _, proceed, err := plugin.Extract(buildConfig, "secret100", "", req)
	matchWarning(t, err, "invalid Content-Type on payload, ignoring payload and continuing with build")
	if !proceed {
		t.Error("Expected 'proceed' return value to be 'true'")
	}
	if revision != nil {
		t.Error("Expected the 'revision' return value to be nil")
	}
}

func TestExtractWithUnparseableContentType(t *testing.T) {
	req, _ := http.NewRequest("POST", "http://someurl.com", errJSON{})
	req.Header.Add("Content-Type", "bad//bad")
	buildConfig := &api.BuildConfig{
		Spec: api.BuildConfigSpec{
			Triggers: []api.BuildTriggerPolicy{
				{
					Type: api.GenericWebHookBuildTriggerType,
					GenericWebHook: &api.WebHookTrigger{
						Secret: "secret100",
					},
				},
			},
			CommonSpec: api.CommonSpec{
				Source: api.BuildSource{
					Git: &api.GitBuildSource{
						Ref: "other",
					},
				},
				Strategy: mockBuildStrategy,
			},
		},
	}
	plugin := New()
	revision, _, proceed, err := plugin.Extract(buildConfig, "secret100", "", req)
	if err == nil || err.Error() != "error parsing Content-Type: mime: expected token after slash" {
		t.Errorf("Unexpected error %v", err)
	}
	if proceed {
		t.Error("Expected 'proceed' return value to be 'false'")
	}
	if revision != nil {
		t.Error("Expected the 'revision' return value to be nil")
	}
}
