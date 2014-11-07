package generic

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"github.com/openshift/origin/pkg/build/api"
)

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

func TestVerifyRequestForMethod(t *testing.T) {
	req := GivenRequest("GET")
	err := verifyRequest(req)
	if err == nil || !strings.Contains(err.Error(), "method") {
		t.Errorf("Expected anything but POST to be an invalid method %s")
	}
}

func TestVerifyRequestForUserAgent(t *testing.T) {
	req := GivenRequest("POST")
	err := verifyRequest(req)
	if err == nil || !strings.Contains(err.Error(), "User-Agent") {
		t.Errorf("Exp. User-Agent to be required %s", err)
	}

	req.Header.Add("User-Agent", "")
	err = verifyRequest(req)
	if err == nil || !strings.Contains(err.Error(), "User-Agent") {
		t.Errorf("Exp. User-Agent to not empty %s", err)
	}

	req.Header.Set("User-Agent", "foobar")
	err = verifyRequest(req)
	if err != nil && strings.Contains(err.Error(), "User-Agent") {
		t.Errorf("Exp. non-empty User-Agent to be valid %s", err)
	}

}

func TestVerifyRequestForContentType(t *testing.T) {
	req := &http.Request{
		Header: http.Header{"User-Agent": {"foobar"}},
		Method: "POST",
	}
	err := verifyRequest(req)
	if err != nil && strings.Contains(err.Error(), "Content-Type") {
		t.Errorf("Exp. a valid request if no payload is posted")
	}

	req.Header.Add("Content-Length", "1")
	err = verifyRequest(req)
	if err == nil || !strings.Contains(err.Error(), "Content-Type") {
		t.Errorf("Exp. ContentType to be required if a payload is posted %s", err)
	}

	req.Header.Add("Content-Type", "X-Whatever")
	err = verifyRequest(req)
	if err == nil || !strings.Contains(err.Error(), "Unsupported Content-Type") {
		t.Errorf("Exp. to only support json payloads %s", err)
	}

	req.Header.Set("Content-Type", "application/json")
	err = verifyRequest(req)
	if err != nil && !strings.Contains(err.Error(), "Unsupported Content-Type") {
		t.Errorf("Exp. to allow json payloads %s", err)
	}
}

func TestExtractWithEmptyPayload(t *testing.T) {
	req := GivenRequest("POST")
	req.Header.Add("User-Agent", "Some User Agent")
	req.Header.Add("Content-Type", "application/json")
	buildConfig := &api.BuildConfig{
		Parameters: api.BuildParameters{
			Source: api.BuildSource{
				Type: api.BuildSourceGit,
				Git: &api.GitBuildSource{
					Ref: "master",
				},
			},
			Strategy: api.BuildStrategy{},
		},
	}
	plugin := New()
	build, proceed, err := plugin.Extract(buildConfig, "", req)
	if err != nil {
		t.Errorf("Expected to be able to trigger a build without a payload error: %s", err)
	}
	if !proceed {
		t.Error("Expected 'proceed' return value to be 'true'")
	}
	if build == nil {
		t.Error("Expected the 'build' return value to not be nil")
	} else {
		if build.Parameters != buildConfig.Parameters {
			t.Errorf("Expected build.Parameters '%s' to be the same as buildConfig.Parameters '%s'", build.Parameters, buildConfig.Parameters)
		}
	}
}

func TestExtractWithUnmatchedRefGitPayload(t *testing.T) {
	req := GivenRequestWithPayload(t)
	buildConfig := &api.BuildConfig{
		Parameters: api.BuildParameters{
			Source: api.BuildSource{
				Type: api.BuildSourceGit,
				Git: &api.GitBuildSource{
					Ref: "asdfkasdfasdfasdfadsfkjhkhkh",
				},
			},
			Strategy: api.BuildStrategy{},
		},
	}
	plugin := New()
	build, proceed, err := plugin.Extract(buildConfig, "", req)

	if err != nil {
		t.Errorf("Unexpected error when triggering build: %s", err)
	}
	if proceed {
		t.Error("Expected 'proceed' return value to be 'false' for unmatched refs")
	}
	if build != nil {
		t.Error("Expected the 'build' return value to be nil since we arent creating a new one")
	}
}

func TestExtractWithGitPayload(t *testing.T) {
	req := GivenRequestWithPayload(t)
	buildConfig := &api.BuildConfig{
		Parameters: api.BuildParameters{
			Source: api.BuildSource{
				Type: api.BuildSourceGit,
				Git: &api.GitBuildSource{
					Ref: "master",
				},
			},
			Strategy: api.BuildStrategy{},
		},
	}
	plugin := New()

	build, proceed, err := plugin.Extract(buildConfig, "", req)

	if err != nil {
		t.Errorf("Expected to be able to trigger a build without a payload error: %s", err)
	}
	if !proceed {
		t.Error("Expected 'proceed' return value to be 'true'")
	}
	if build == nil {
		t.Error("Expected the 'build' return value to not be nil")
	} else {
		if build.Parameters.Source != buildConfig.Parameters.Source {
			t.Errorf("Expected build.parameters.source '%s' to be the same as buildConfig.parameters.source '%s'", build.Parameters.Source, buildConfig.Parameters.Source)
		}
		if build.Parameters.Strategy != buildConfig.Parameters.Strategy {
			t.Errorf("Expected build.Parameters.Strategy '%s' to be the same as buildConfig.Parameters.Strategy '%s'", build.Parameters.Strategy, buildConfig.Parameters.Strategy)
		}
		if build.Parameters.Revision == nil {
			t.Error("Expected build.Parameters.Revision to not be nil")
		}
	}
}
