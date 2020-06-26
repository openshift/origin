package docker

import (
	"net/http"
	"net/url"
	"reflect"
	"testing"
)

func TestRemoveContainer(t *testing.T) {
	t.Parallel()
	fakeRT := &FakeRoundTripper{message: "", status: http.StatusOK}
	client := newTestClient(fakeRT)
	id := "4fa6e0f0c6786287e131c3852c58a2e01cc697a68231826813597e4994f1d6e2"
	opts := RemoveContainerOptions{ID: id}
	err := client.RemoveContainer(opts)
	if err != nil {
		t.Fatal(err)
	}
	req := fakeRT.requests[0]
	if req.Method != http.MethodDelete {
		t.Errorf("RemoveContainer(%q): wrong HTTP method. Want %q. Got %q.", id, http.MethodDelete, req.Method)
	}
	expectedURL, _ := url.Parse(client.getURL("/containers/" + id))
	if gotPath := req.URL.Path; gotPath != expectedURL.Path {
		t.Errorf("RemoveContainer(%q): Wrong path in request. Want %q. Got %q.", id, expectedURL.Path, gotPath)
	}
}

func TestRemoveContainerRemoveVolumes(t *testing.T) {
	t.Parallel()
	fakeRT := &FakeRoundTripper{message: "", status: http.StatusOK}
	client := newTestClient(fakeRT)
	id := "4fa6e0f0c6786287e131c3852c58a2e01cc697a68231826813597e4994f1d6e2"
	opts := RemoveContainerOptions{ID: id, RemoveVolumes: true}
	err := client.RemoveContainer(opts)
	if err != nil {
		t.Fatal(err)
	}
	req := fakeRT.requests[0]
	params := map[string][]string(req.URL.Query())
	expected := map[string][]string{"v": {"1"}}
	if !reflect.DeepEqual(params, expected) {
		t.Errorf("RemoveContainer(%q): wrong parameters. Want %#v. Got %#v.", id, expected, params)
	}
}

func TestRemoveContainerNotFound(t *testing.T) {
	t.Parallel()
	client := newTestClient(&FakeRoundTripper{message: "no such container", status: http.StatusNotFound})
	err := client.RemoveContainer(RemoveContainerOptions{ID: "a2334"})
	expectNoSuchContainer(t, "a2334", err)
}
