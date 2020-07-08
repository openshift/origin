package docker

import (
	"net/http"
	"net/url"
	"testing"
)

func TestRestartContainer(t *testing.T) {
	t.Parallel()
	fakeRT := &FakeRoundTripper{message: "", status: http.StatusNoContent}
	client := newTestClient(fakeRT)
	id := "4fa6e0f0c6786287e131c3852c58a2e01cc697a68231826813597e4994f1d6e2"
	err := client.RestartContainer(id, 10)
	if err != nil {
		t.Fatal(err)
	}
	req := fakeRT.requests[0]
	if req.Method != http.MethodPost {
		t.Errorf("RestartContainer(%q, 10): wrong HTTP method. Want %q. Got %q.", id, http.MethodPost, req.Method)
	}
	expectedURL, _ := url.Parse(client.getURL("/containers/" + id + "/restart"))
	if gotPath := req.URL.Path; gotPath != expectedURL.Path {
		t.Errorf("RestartContainer(%q, 10): Wrong path in request. Want %q. Got %q.", id, expectedURL.Path, gotPath)
	}
}

func TestRestartContainerNotFound(t *testing.T) {
	t.Parallel()
	client := newTestClient(&FakeRoundTripper{message: "no such container", status: http.StatusNotFound})
	err := client.RestartContainer("a2334", 10)
	expectNoSuchContainer(t, "a2334", err)
}

func TestAlwaysRestart(t *testing.T) {
	t.Parallel()
	policy := AlwaysRestart()
	if policy.Name != "always" {
		t.Errorf("AlwaysRestart(): wrong policy name. Want %q. Got %q", "always", policy.Name)
	}
	if policy.MaximumRetryCount != 0 {
		t.Errorf("AlwaysRestart(): wrong MaximumRetryCount. Want 0. Got %d", policy.MaximumRetryCount)
	}
}

func TestRestartOnFailure(t *testing.T) {
	t.Parallel()
	const retry = 5
	policy := RestartOnFailure(retry)
	if policy.Name != "on-failure" {
		t.Errorf("RestartOnFailure(%d): wrong policy name. Want %q. Got %q", retry, "on-failure", policy.Name)
	}
	if policy.MaximumRetryCount != retry {
		t.Errorf("RestartOnFailure(%d): wrong MaximumRetryCount. Want %d. Got %d", retry, retry, policy.MaximumRetryCount)
	}
}

func TestRestartUnlessStopped(t *testing.T) {
	t.Parallel()
	policy := RestartUnlessStopped()
	if policy.Name != "unless-stopped" {
		t.Errorf("RestartUnlessStopped(): wrong policy name. Want %q. Got %q", "unless-stopped", policy.Name)
	}
	if policy.MaximumRetryCount != 0 {
		t.Errorf("RestartUnlessStopped(): wrong MaximumRetryCount. Want 0. Got %d", policy.MaximumRetryCount)
	}
}

func TestNeverRestart(t *testing.T) {
	t.Parallel()
	policy := NeverRestart()
	if policy.Name != "no" {
		t.Errorf("NeverRestart(): wrong policy name. Want %q. Got %q", "always", policy.Name)
	}
	if policy.MaximumRetryCount != 0 {
		t.Errorf("NeverRestart(): wrong MaximumRetryCount. Want 0. Got %d", policy.MaximumRetryCount)
	}
}
