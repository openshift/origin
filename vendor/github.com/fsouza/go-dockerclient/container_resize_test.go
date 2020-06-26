package docker

import (
	"net/http"
	"net/url"
	"reflect"
	"testing"
)

func TestResizeContainerTTY(t *testing.T) {
	t.Parallel()
	fakeRT := &FakeRoundTripper{message: "", status: http.StatusOK}
	client := newTestClient(fakeRT)
	id := "4fa6e0f0c6786287e131c3852c58a2e01cc697a68231826813597e4994f1d6e2"
	err := client.ResizeContainerTTY(id, 40, 80)
	if err != nil {
		t.Fatal(err)
	}
	req := fakeRT.requests[0]
	if req.Method != http.MethodPost {
		t.Errorf("ResizeContainerTTY(%q): wrong HTTP method. Want %q. Got %q.", id, http.MethodPost, req.Method)
	}
	expectedURL, _ := url.Parse(client.getURL("/containers/" + id + "/resize"))
	if gotPath := req.URL.Path; gotPath != expectedURL.Path {
		t.Errorf("ResizeContainerTTY(%q): Wrong path in request. Want %q. Got %q.", id, expectedURL.Path, gotPath)
	}
	got := map[string][]string(req.URL.Query())
	expectedParams := map[string][]string{
		"w": {"80"},
		"h": {"40"},
	}
	if !reflect.DeepEqual(got, expectedParams) {
		t.Errorf("Expected %#v, got %#v.", expectedParams, got)
	}
}
