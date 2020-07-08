package docker

import (
	"encoding/json"
	"net/http"
	"net/url"
	"reflect"
	"testing"
)

func TestContainerChanges(t *testing.T) {
	t.Parallel()
	jsonChanges := `[
     {
             "Path":"/dev",
             "Kind":0
     },
     {
             "Path":"/dev/kmsg",
             "Kind":1
     },
     {
             "Path":"/test",
             "Kind":1
     }
]`
	var expected []Change
	err := json.Unmarshal([]byte(jsonChanges), &expected)
	if err != nil {
		t.Fatal(err)
	}
	fakeRT := &FakeRoundTripper{message: jsonChanges, status: http.StatusOK}
	client := newTestClient(fakeRT)
	id := "4fa6e0f0c678"
	changes, err := client.ContainerChanges(id)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(changes, expected) {
		t.Errorf("ContainerChanges(%q): Expected %#v. Got %#v.", id, expected, changes)
	}
	expectedURL, _ := url.Parse(client.getURL("/containers/4fa6e0f0c678/changes"))
	if gotPath := fakeRT.requests[0].URL.Path; gotPath != expectedURL.Path {
		t.Errorf("ContainerChanges(%q): Wrong path in request. Want %q. Got %q.", id, expectedURL.Path, gotPath)
	}
}

func TestContainerChangesFailure(t *testing.T) {
	t.Parallel()
	client := newTestClient(&FakeRoundTripper{message: "server error", status: 500})
	expected := Error{Status: 500, Message: "server error"}
	changes, err := client.ContainerChanges("abe033")
	if changes != nil {
		t.Errorf("ContainerChanges: Expected <nil> changes, got %#v", changes)
	}
	if !reflect.DeepEqual(expected, *err.(*Error)) {
		t.Errorf("ContainerChanges: Wrong error information. Want %#v. Got %#v.", expected, err)
	}
}

func TestContainerChangesNotFound(t *testing.T) {
	t.Parallel()
	const containerID = "abe033"
	client := newTestClient(&FakeRoundTripper{message: "no such container", status: 404})
	changes, err := client.ContainerChanges(containerID)
	if changes != nil {
		t.Errorf("ContainerChanges: Expected <nil> changes, got %#v", changes)
	}
	expectNoSuchContainer(t, containerID, err)
}
