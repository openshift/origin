package docker

import (
	"net/http"
	"net/url"
	"testing"
)

func TestPauseContainer(t *testing.T) {
	t.Parallel()
	fakeRT := &FakeRoundTripper{message: "", status: http.StatusNoContent}
	client := newTestClient(fakeRT)
	id := "4fa6e0f0c6786287e131c3852c58a2e01cc697a68231826813597e4994f1d6e2"
	err := client.PauseContainer(id)
	if err != nil {
		t.Fatal(err)
	}
	req := fakeRT.requests[0]
	if req.Method != http.MethodPost {
		t.Errorf("PauseContainer(%q): wrong HTTP method. Want %q. Got %q.", id, http.MethodPost, req.Method)
	}
	expectedURL, _ := url.Parse(client.getURL("/containers/" + id + "/pause"))
	if gotPath := req.URL.Path; gotPath != expectedURL.Path {
		t.Errorf("PauseContainer(%q): Wrong path in request. Want %q. Got %q.", id, expectedURL.Path, gotPath)
	}
}

func TestPauseContainerNotFound(t *testing.T) {
	t.Parallel()
	client := newTestClient(&FakeRoundTripper{message: "no such container", status: http.StatusNotFound})
	err := client.PauseContainer("a2334")
	expectNoSuchContainer(t, "a2334", err)
}
