package docker

import (
	"encoding/json"
	"net/http"
	"net/url"
	"reflect"
	"testing"
)

func TestUpdateContainer(t *testing.T) {
	t.Parallel()
	fakeRT := &FakeRoundTripper{message: "", status: http.StatusOK}
	client := newTestClient(fakeRT)
	id := "4fa6e0f0c6786287e131c3852c58a2e01cc697a68231826813597e4994f1d6e2"
	update := UpdateContainerOptions{Memory: 12345, CpusetMems: "0,1"}
	err := client.UpdateContainer(id, update)
	if err != nil {
		t.Fatal(err)
	}
	req := fakeRT.requests[0]
	if req.Method != http.MethodPost {
		t.Errorf("UpdateContainer: wrong HTTP method. Want %q. Got %q.", http.MethodPost, req.Method)
	}
	expectedURL, _ := url.Parse(client.getURL("/containers/" + id + "/update"))
	if gotPath := req.URL.Path; gotPath != expectedURL.Path {
		t.Errorf("UpdateContainer: Wrong path in request. Want %q. Got %q.", expectedURL.Path, gotPath)
	}
	expectedContentType := "application/json"
	if contentType := req.Header.Get("Content-Type"); contentType != expectedContentType {
		t.Errorf("UpdateContainer: Wrong content-type in request. Want %q. Got %q.", expectedContentType, contentType)
	}
	var out UpdateContainerOptions
	if err := json.NewDecoder(req.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(out, update) {
		t.Errorf("UpdateContainer: wrong body, got: %#v, want %#v", out, update)
	}
}
