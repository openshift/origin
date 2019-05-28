package generic

import (
	"bytes"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	buildv1 "github.com/openshift/api/build/v1"
)

var mockBuildStrategy = buildv1.BuildStrategy{
	SourceStrategy: &buildv1.SourceBuildStrategy{
		From: corev1.ObjectReference{
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

	if status.ErrStatus.Status != metav1.StatusSuccess {
		t.Errorf("Unexpected response status %v, expected %v", status.ErrStatus.Status, metav1.StatusSuccess)
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
	buildConfig := &buildv1.BuildConfig{
		Spec: buildv1.BuildConfigSpec{
			Triggers: []buildv1.BuildTriggerPolicy{
				{
					Type: buildv1.GenericWebHookBuildTriggerType,
					GenericWebHook: &buildv1.WebHookTrigger{
						Secret: "secret100",
					},
				},
			},
		},
	}
	plugin := New()
	revision, _, _, proceed, err := plugin.Extract(buildConfig, buildConfig.Spec.Triggers[0].GenericWebHook, req)

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

type emptyReader struct{}

func (_ emptyReader) Read(p []byte) (n int, err error) {
	return 0, io.EOF
}

func TestExtractWithEmptyPayload(t *testing.T) {
	req, _ := http.NewRequest("POST", "http://someurl.com", emptyReader{})
	req.Header.Add("Content-Type", "application/json")
	buildConfig := &buildv1.BuildConfig{
		Spec: buildv1.BuildConfigSpec{

			Triggers: []buildv1.BuildTriggerPolicy{
				{
					Type: buildv1.GenericWebHookBuildTriggerType,
					GenericWebHook: &buildv1.WebHookTrigger{
						Secret: "secret100",
					},
				},
			},
			CommonSpec: buildv1.CommonSpec{
				Source: buildv1.BuildSource{
					Git: &buildv1.GitBuildSource{
						Ref: "master",
					},
				},
				Strategy: mockBuildStrategy,
			},
		},
	}
	plugin := New()
	revision, _, _, proceed, err := plugin.Extract(buildConfig, buildConfig.Spec.Triggers[0].GenericWebHook, req)
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
	buildConfig := &buildv1.BuildConfig{
		Spec: buildv1.BuildConfigSpec{
			Triggers: []buildv1.BuildTriggerPolicy{
				{
					Type: buildv1.GenericWebHookBuildTriggerType,
					GenericWebHook: &buildv1.WebHookTrigger{
						Secret: "secret100",
					},
				},
			},
			CommonSpec: buildv1.CommonSpec{
				Source: buildv1.BuildSource{
					Git: &buildv1.GitBuildSource{
						Ref: "asdfkasdfasdfasdfadsfkjhkhkh",
					},
				},
				Strategy: mockBuildStrategy,
			},
		},
	}
	plugin := New()
	build, _, _, proceed, err := plugin.Extract(buildConfig, buildConfig.Spec.Triggers[0].GenericWebHook, req)

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
	buildConfig := &buildv1.BuildConfig{
		Spec: buildv1.BuildConfigSpec{
			Triggers: []buildv1.BuildTriggerPolicy{
				{
					Type: buildv1.GenericWebHookBuildTriggerType,
					GenericWebHook: &buildv1.WebHookTrigger{
						Secret: "secret100",
					},
				},
			},
			CommonSpec: buildv1.CommonSpec{
				Source: buildv1.BuildSource{
					Git: &buildv1.GitBuildSource{
						Ref: "master",
					},
				},
				Strategy: mockBuildStrategy,
			},
		},
	}
	plugin := New()
	revision, _, _, proceed, err := plugin.Extract(buildConfig, buildConfig.Spec.Triggers[0].GenericWebHook, req)

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
	buildConfig := &buildv1.BuildConfig{
		Spec: buildv1.BuildConfigSpec{
			Triggers: []buildv1.BuildTriggerPolicy{
				{
					Type: buildv1.GenericWebHookBuildTriggerType,
					GenericWebHook: &buildv1.WebHookTrigger{
						Secret: "secret100",
					},
				},
			},
			CommonSpec: buildv1.CommonSpec{
				Source: buildv1.BuildSource{
					Git: &buildv1.GitBuildSource{
						Ref: "master",
					},
				},
				Strategy: mockBuildStrategy,
			},
		},
	}
	plugin := New()
	revision, _, _, proceed, err := plugin.Extract(buildConfig, buildConfig.Spec.Triggers[0].GenericWebHook, req)

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
	buildConfig := &buildv1.BuildConfig{
		Spec: buildv1.BuildConfigSpec{
			Triggers: []buildv1.BuildTriggerPolicy{
				{
					Type: buildv1.GenericWebHookBuildTriggerType,
					GenericWebHook: &buildv1.WebHookTrigger{
						Secret: "secret100",
					},
				},
			},
			CommonSpec: buildv1.CommonSpec{
				Source: buildv1.BuildSource{
					Git: &buildv1.GitBuildSource{
						Ref: "master",
					},
				},
				Strategy: mockBuildStrategy,
			},
		},
	}
	plugin := New()
	revision, _, _, proceed, err := plugin.Extract(buildConfig, buildConfig.Spec.Triggers[0].GenericWebHook, req)

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
	buildConfig := &buildv1.BuildConfig{
		Spec: buildv1.BuildConfigSpec{
			Triggers: []buildv1.BuildTriggerPolicy{
				{
					Type: buildv1.GenericWebHookBuildTriggerType,
					GenericWebHook: &buildv1.WebHookTrigger{
						Secret: "secret100",
					},
				},
			},
			CommonSpec: buildv1.CommonSpec{
				Source: buildv1.BuildSource{
					Git: &buildv1.GitBuildSource{
						Ref: "other",
					},
				},
				Strategy: mockBuildStrategy,
			},
		},
	}
	plugin := New()
	revision, _, _, proceed, err := plugin.Extract(buildConfig, buildConfig.Spec.Triggers[0].GenericWebHook, req)

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
	buildConfig := &buildv1.BuildConfig{
		Spec: buildv1.BuildConfigSpec{
			Triggers: []buildv1.BuildTriggerPolicy{
				{
					Type: buildv1.GenericWebHookBuildTriggerType,
					GenericWebHook: &buildv1.WebHookTrigger{
						Secret:   "secret100",
						AllowEnv: true,
					},
				},
			},
			CommonSpec: buildv1.CommonSpec{
				Source: buildv1.BuildSource{
					Git: &buildv1.GitBuildSource{
						Ref: "master",
					},
				},
				Strategy: mockBuildStrategy,
			},
		},
	}
	plugin := New()
	revision, envvars, _, proceed, err := plugin.Extract(buildConfig, buildConfig.Spec.Triggers[0].GenericWebHook, req)

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
	buildConfig := &buildv1.BuildConfig{
		Spec: buildv1.BuildConfigSpec{
			Triggers: []buildv1.BuildTriggerPolicy{
				{
					Type: buildv1.GenericWebHookBuildTriggerType,
					GenericWebHook: &buildv1.WebHookTrigger{
						Secret:   "secret100",
						AllowEnv: true,
					},
				},
			},
			CommonSpec: buildv1.CommonSpec{
				Source: buildv1.BuildSource{
					Git: &buildv1.GitBuildSource{
						Ref: "master",
					},
				},
				Strategy: mockBuildStrategy,
			},
		},
	}
	plugin := New()
	revision, envvars, _, proceed, err := plugin.Extract(buildConfig, buildConfig.Spec.Triggers[0].GenericWebHook, req)

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
	buildConfig := &buildv1.BuildConfig{
		Spec: buildv1.BuildConfigSpec{
			Triggers: []buildv1.BuildTriggerPolicy{
				{
					Type: buildv1.GenericWebHookBuildTriggerType,
					GenericWebHook: &buildv1.WebHookTrigger{
						Secret: "secret100",
					},
				},
			},
			CommonSpec: buildv1.CommonSpec{
				Source: buildv1.BuildSource{
					Git: &buildv1.GitBuildSource{
						Ref: "master",
					},
				},
				Strategy: mockBuildStrategy,
			},
		},
	}
	plugin := New()
	revision, envvars, _, proceed, err := plugin.Extract(buildConfig, buildConfig.Spec.Triggers[0].GenericWebHook, req)

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
	buildConfig := &buildv1.BuildConfig{
		Spec: buildv1.BuildConfigSpec{
			Triggers: []buildv1.BuildTriggerPolicy{
				{
					Type: buildv1.GenericWebHookBuildTriggerType,
					GenericWebHook: &buildv1.WebHookTrigger{
						Secret: "secret100",
					},
				},
			},
			CommonSpec: buildv1.CommonSpec{
				Source: buildv1.BuildSource{
					Git: &buildv1.GitBuildSource{},
				},
				Strategy: mockBuildStrategy,
			},
		},
	}
	plugin := New()
	_, _, _, proceed, err := plugin.Extract(buildConfig, buildConfig.Spec.Triggers[0].GenericWebHook, req)

	matchWarning(t, err, "no git information found in payload, ignoring and continuing with build")
	if !proceed {
		t.Error("Expected 'proceed' return value to be 'true'")
	}
}
func TestNonJsonPush(t *testing.T) {
	req, _ := http.NewRequest("POST", "http://someurl.com", nil)
	req.Header.Add("Content-Type", "*/*")

	buildConfig := &buildv1.BuildConfig{
		Spec: buildv1.BuildConfigSpec{
			Triggers: []buildv1.BuildTriggerPolicy{
				{
					Type: buildv1.GenericWebHookBuildTriggerType,
					GenericWebHook: &buildv1.WebHookTrigger{
						Secret: "secret100",
					},
				},
			},
			CommonSpec: buildv1.CommonSpec{
				Source: buildv1.BuildSource{
					Git: &buildv1.GitBuildSource{},
				},
				Strategy: mockBuildStrategy,
			},
		},
	}
	plugin := New()
	_, _, _, proceed, err := plugin.Extract(buildConfig, buildConfig.Spec.Triggers[0].GenericWebHook, req)

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
	buildConfig := &buildv1.BuildConfig{
		Spec: buildv1.BuildConfigSpec{
			Triggers: []buildv1.BuildTriggerPolicy{
				{
					Type: buildv1.GenericWebHookBuildTriggerType,
					GenericWebHook: &buildv1.WebHookTrigger{
						Secret: "secret100",
					},
				},
			},
			CommonSpec: buildv1.CommonSpec{
				Source: buildv1.BuildSource{
					Git: &buildv1.GitBuildSource{
						Ref: "other",
					},
				},
				Strategy: mockBuildStrategy,
			},
		},
	}
	plugin := New()
	revision, _, _, proceed, err := plugin.Extract(buildConfig, buildConfig.Spec.Triggers[0].GenericWebHook, req)
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
	buildConfig := &buildv1.BuildConfig{
		Spec: buildv1.BuildConfigSpec{
			Triggers: []buildv1.BuildTriggerPolicy{
				{
					Type: buildv1.GenericWebHookBuildTriggerType,
					GenericWebHook: &buildv1.WebHookTrigger{
						Secret: "secret100",
					},
				},
			},
			CommonSpec: buildv1.CommonSpec{
				Source: buildv1.BuildSource{
					Git: &buildv1.GitBuildSource{
						Ref: "other",
					},
				},
				Strategy: mockBuildStrategy,
			},
		},
	}
	plugin := New()
	revision, _, _, proceed, err := plugin.Extract(buildConfig, buildConfig.Spec.Triggers[0].GenericWebHook, req)
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
	buildConfig := &buildv1.BuildConfig{
		Spec: buildv1.BuildConfigSpec{
			Triggers: []buildv1.BuildTriggerPolicy{
				{
					Type: buildv1.GenericWebHookBuildTriggerType,
					GenericWebHook: &buildv1.WebHookTrigger{
						Secret: "secret100",
					},
				},
			},
			CommonSpec: buildv1.CommonSpec{
				Source: buildv1.BuildSource{
					Git: &buildv1.GitBuildSource{
						Ref: "other",
					},
				},
				Strategy: mockBuildStrategy,
			},
		},
	}
	plugin := New()
	revision, _, _, proceed, err := plugin.Extract(buildConfig, buildConfig.Spec.Triggers[0].GenericWebHook, req)
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
	buildConfig := &buildv1.BuildConfig{
		Spec: buildv1.BuildConfigSpec{
			Triggers: []buildv1.BuildTriggerPolicy{
				{
					Type: buildv1.GenericWebHookBuildTriggerType,
					GenericWebHook: &buildv1.WebHookTrigger{
						Secret: "secret100",
					},
				},
			},
			CommonSpec: buildv1.CommonSpec{
				Source: buildv1.BuildSource{
					Git: &buildv1.GitBuildSource{
						Ref: "other",
					},
				},
				Strategy: mockBuildStrategy,
			},
		},
	}
	plugin := New()
	revision, _, _, proceed, err := plugin.Extract(buildConfig, buildConfig.Spec.Triggers[0].GenericWebHook, req)
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
