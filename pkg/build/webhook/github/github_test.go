package github

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/build/webhook"
	"github.com/openshift/origin/pkg/client"
)

type osClient struct {
	client.Fake
}

func (_ *osClient) GetBuildConfig(ctx kapi.Context, id string) (result *api.BuildConfig, err error) {
	return &api.BuildConfig{Secret: "secret101"}, nil
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

func TestJsonPingEventError(t *testing.T) {
	server := httptest.NewServer(webhook.NewController(&osClient{}, map[string]webhook.Plugin{"github": New()}))
	defer server.Close()

	post("ping", []byte{}, server.URL+"/build100/secret101/github", http.StatusBadRequest, t)
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
