package docker

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"reflect"
	"testing"
)

func TestCommitContainer(t *testing.T) {
	t.Parallel()
	response := `{"Id":"596069db4bf5"}`
	client := newTestClient(&FakeRoundTripper{message: response, status: http.StatusOK})
	id := "596069db4bf5"
	image, err := client.CommitContainer(CommitContainerOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if image.ID != id {
		t.Errorf("CommitContainer: Wrong image id. Want %q. Got %q.", id, image.ID)
	}
}

func TestCommitContainerParams(t *testing.T) {
	t.Parallel()
	cfg := Config{Memory: 67108864}
	json, _ := json.Marshal(&cfg)
	tests := []struct {
		input  CommitContainerOptions
		params map[string][]string
		body   []byte
	}{
		{CommitContainerOptions{}, map[string][]string{}, nil},
		{CommitContainerOptions{Container: "44c004db4b17"}, map[string][]string{"container": {"44c004db4b17"}}, nil},
		{
			CommitContainerOptions{Container: "44c004db4b17", Repository: "tsuru/python", Message: "something"},
			map[string][]string{"container": {"44c004db4b17"}, "repo": {"tsuru/python"}, "comment": {"something"}},
			nil,
		},
		{
			CommitContainerOptions{Container: "44c004db4b17", Run: &cfg},
			map[string][]string{"container": {"44c004db4b17"}},
			json,
		},
	}
	const expectedPath = "/commit"
	for _, tt := range tests {
		test := tt
		t.Run("", func(t *testing.T) {
			t.Parallel()
			fakeRT := &FakeRoundTripper{message: "{}", status: http.StatusOK}
			client := newTestClient(fakeRT)
			if _, err := client.CommitContainer(test.input); err != nil {
				t.Error(err)
			}
			got := map[string][]string(fakeRT.requests[0].URL.Query())
			if !reflect.DeepEqual(got, test.params) {
				t.Errorf("Expected %#v, got %#v.", test.params, got)
			}
			if path := fakeRT.requests[0].URL.Path; path != expectedPath {
				t.Errorf("Wrong path on request. Want %q. Got %q.", expectedPath, path)
			}
			if meth := fakeRT.requests[0].Method; meth != http.MethodPost {
				t.Errorf("Wrong HTTP method. Want POST. Got %s.", meth)
			}
			if test.body != nil {
				if requestBody, err := ioutil.ReadAll(fakeRT.requests[0].Body); err == nil {
					if !bytes.Equal(requestBody, test.body) {
						t.Errorf("Expected body %#v, got %#v", test.body, requestBody)
					}
				} else {
					t.Errorf("Error reading request body: %#v", err)
				}
			}
		})
	}
}

func TestCommitContainerFailure(t *testing.T) {
	t.Parallel()
	client := newTestClient(&FakeRoundTripper{message: "no such container", status: http.StatusInternalServerError})
	_, err := client.CommitContainer(CommitContainerOptions{})
	if err == nil {
		t.Error("Expected non-nil error, got <nil>.")
	}
}

func TestCommitContainerNotFound(t *testing.T) {
	t.Parallel()
	client := newTestClient(&FakeRoundTripper{message: "no such container", status: http.StatusNotFound})
	_, err := client.CommitContainer(CommitContainerOptions{})
	expectNoSuchContainer(t, "", err)
}
