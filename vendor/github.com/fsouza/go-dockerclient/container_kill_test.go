package docker

import (
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"testing"
)

func TestKillContainer(t *testing.T) {
	t.Parallel()
	fakeRT := &FakeRoundTripper{message: "", status: http.StatusNoContent}
	client := newTestClient(fakeRT)
	id := "4fa6e0f0c6786287e131c3852c58a2e01cc697a68231826813597e4994f1d6e2"
	err := client.KillContainer(KillContainerOptions{ID: id})
	if err != nil {
		t.Fatal(err)
	}
	req := fakeRT.requests[0]
	if req.Method != http.MethodPost {
		t.Errorf("KillContainer(%q): wrong HTTP method. Want %q. Got %q.", id, http.MethodPost, req.Method)
	}
	expectedURL, _ := url.Parse(client.getURL("/containers/" + id + "/kill"))
	if gotPath := req.URL.Path; gotPath != expectedURL.Path {
		t.Errorf("KillContainer(%q): Wrong path in request. Want %q. Got %q.", id, expectedURL.Path, gotPath)
	}
}

func TestKillContainerSignal(t *testing.T) {
	t.Parallel()
	fakeRT := &FakeRoundTripper{message: "", status: http.StatusNoContent}
	client := newTestClient(fakeRT)
	id := "4fa6e0f0c6786287e131c3852c58a2e01cc697a68231826813597e4994f1d6e2"
	err := client.KillContainer(KillContainerOptions{ID: id, Signal: SIGTERM})
	if err != nil {
		t.Fatal(err)
	}
	req := fakeRT.requests[0]
	if req.Method != http.MethodPost {
		t.Errorf("KillContainer(%q): wrong HTTP method. Want %q. Got %q.", id, http.MethodPost, req.Method)
	}
	if signal := req.URL.Query().Get("signal"); signal != "15" {
		t.Errorf("KillContainer(%q): Wrong query string in request. Want %q. Got %q.", id, "15", signal)
	}
}

func TestKillContainerNotFound(t *testing.T) {
	t.Parallel()
	client := newTestClient(&FakeRoundTripper{message: "no such container", status: http.StatusNotFound})
	err := client.KillContainer(KillContainerOptions{ID: "a2334"})
	expectNoSuchContainer(t, "a2334", err)
}

func TestKillContainerNotRunning(t *testing.T) {
	t.Parallel()
	id := "abcd1234567890"
	msg := fmt.Sprintf("Cannot kill container: %[1]s: Container %[1]s is not running", id)
	client := newTestClient(&FakeRoundTripper{message: msg, status: http.StatusConflict})
	err := client.KillContainer(KillContainerOptions{ID: id})
	expected := &ContainerNotRunning{ID: id}
	if !reflect.DeepEqual(err, expected) {
		t.Errorf("KillContainer: Wrong error returned. Want %#v. Got %#v.", expected, err)
	}
}
