package docker

import (
	"net/http"
	"net/url"
	"testing"
)

func TestRenameContainer(t *testing.T) {
	t.Parallel()
	fakeRT := &FakeRoundTripper{message: "", status: http.StatusOK}
	client := newTestClient(fakeRT)
	opts := RenameContainerOptions{ID: "something_old", Name: "something_new"}
	err := client.RenameContainer(opts)
	if err != nil {
		t.Fatal(err)
	}
	req := fakeRT.requests[0]
	if req.Method != http.MethodPost {
		t.Errorf("RenameContainer: wrong HTTP method. Want %q. Got %q.", http.MethodPost, req.Method)
	}
	expectedURL, _ := url.Parse(client.getURL("/containers/something_old/rename?name=something_new"))
	if gotPath := req.URL.Path; gotPath != expectedURL.Path {
		t.Errorf("RenameContainer: Wrong path in request. Want %q. Got %q.", expectedURL.Path, gotPath)
	}
	expectedValues := expectedURL.Query()["name"]
	actualValues := req.URL.Query()["name"]
	if len(actualValues) != 1 || expectedValues[0] != actualValues[0] {
		t.Errorf("RenameContainer: Wrong params in request. Want %q. Got %q.", expectedValues, actualValues)
	}
}
