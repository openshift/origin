package docker

import (
	"context"
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"
	"reflect"
	"testing"
	"time"
)

func TestStartContainer(t *testing.T) {
	t.Parallel()
	fakeRT := &FakeRoundTripper{message: "", status: http.StatusOK}
	client := newTestClient(fakeRT)
	id := "4fa6e0f0c6786287e131c3852c58a2e01cc697a68231826813597e4994f1d6e2"
	err := client.StartContainer(id, &HostConfig{})
	if err != nil {
		t.Fatal(err)
	}
	req := fakeRT.requests[0]
	if req.Method != http.MethodPost {
		t.Errorf("StartContainer(%q): wrong HTTP method. Want %q. Got %q.", id, http.MethodPost, req.Method)
	}
	expectedURL, _ := url.Parse(client.getURL("/containers/" + id + "/start"))
	if gotPath := req.URL.Path; gotPath != expectedURL.Path {
		t.Errorf("StartContainer(%q): Wrong path in request. Want %q. Got %q.", id, expectedURL.Path, gotPath)
	}
	expectedContentType := "application/json"
	if contentType := req.Header.Get("Content-Type"); contentType != expectedContentType {
		t.Errorf("StartContainer(%q): Wrong content-type in request. Want %q. Got %q.", id, expectedContentType, contentType)
	}
}

func TestStartContainerHostConfigAPI124(t *testing.T) {
	t.Parallel()
	fakeRT := &FakeRoundTripper{message: "", status: http.StatusOK}
	client := newTestClient(fakeRT)
	client.serverAPIVersion = apiVersion124
	id := "4fa6e0f0c6786287e131c3852c58a2e01cc697a68231826813597e4994f1d6e2"
	err := client.StartContainer(id, &HostConfig{})
	if err != nil {
		t.Fatal(err)
	}
	req := fakeRT.requests[0]
	if req.Method != http.MethodPost {
		t.Errorf("StartContainer(%q): wrong HTTP method. Want %q. Got %q.", id, http.MethodPost, req.Method)
	}
	expectedURL, _ := url.Parse(client.getURL("/containers/" + id + "/start"))
	if gotPath := req.URL.Path; gotPath != expectedURL.Path {
		t.Errorf("StartContainer(%q): Wrong path in request. Want %q. Got %q.", id, expectedURL.Path, gotPath)
	}
	notAcceptedContentType := "application/json"
	if contentType := req.Header.Get("Content-Type"); contentType == notAcceptedContentType {
		t.Errorf("StartContainer(%q): Unepected %q Content-Type in request.", id, contentType)
	}
	if req.Body != nil {
		data, _ := ioutil.ReadAll(req.Body)
		t.Errorf("StartContainer(%q): Unexpected data sent: %s", id, data)
	}
}

func TestStartContainerNilHostConfig(t *testing.T) {
	t.Parallel()
	fakeRT := &FakeRoundTripper{message: "", status: http.StatusOK}
	client := newTestClient(fakeRT)
	id := "4fa6e0f0c6786287e131c3852c58a2e01cc697a68231826813597e4994f1d6e2"
	err := client.StartContainer(id, nil)
	if err != nil {
		t.Fatal(err)
	}
	req := fakeRT.requests[0]
	if req.Method != http.MethodPost {
		t.Errorf("StartContainer(%q): wrong HTTP method. Want %q. Got %q.", id, http.MethodPost, req.Method)
	}
	expectedURL, _ := url.Parse(client.getURL("/containers/" + id + "/start"))
	if gotPath := req.URL.Path; gotPath != expectedURL.Path {
		t.Errorf("StartContainer(%q): Wrong path in request. Want %q. Got %q.", id, expectedURL.Path, gotPath)
	}
	expectedContentType := "application/json"
	if contentType := req.Header.Get("Content-Type"); contentType != expectedContentType {
		t.Errorf("StartContainer(%q): Wrong content-type in request. Want %q. Got %q.", id, expectedContentType, contentType)
	}
	var buf [4]byte
	req.Body.Read(buf[:])
	if string(buf[:]) != "null" {
		t.Errorf("Startcontainer(%q): Wrong body. Want null. Got %s", id, buf[:])
	}
}

func TestStartContainerWithContext(t *testing.T) {
	t.Parallel()
	fakeRT := &FakeRoundTripper{message: "", status: http.StatusOK}
	client := newTestClient(fakeRT)
	id := "4fa6e0f0c6786287e131c3852c58a2e01cc697a68231826813597e4994f1d6e2"

	ctx, cancel := context.WithTimeout(context.TODO(), 1*time.Second)
	defer cancel()

	startError := make(chan error)
	go func() {
		startError <- client.StartContainerWithContext(id, &HostConfig{}, ctx)
	}()
	select {
	case err := <-startError:
		if err != nil {
			t.Fatal(err)
		}
		req := fakeRT.requests[0]
		if req.Method != http.MethodPost {
			t.Errorf("StartContainer(%q): wrong HTTP method. Want %q. Got %q.", id, http.MethodPost, req.Method)
		}
		expectedURL, _ := url.Parse(client.getURL("/containers/" + id + "/start"))
		if gotPath := req.URL.Path; gotPath != expectedURL.Path {
			t.Errorf("StartContainer(%q): Wrong path in request. Want %q. Got %q.", id, expectedURL.Path, gotPath)
		}
		expectedContentType := "application/json"
		if contentType := req.Header.Get("Content-Type"); contentType != expectedContentType {
			t.Errorf("StartContainer(%q): Wrong content-type in request. Want %q. Got %q.", id, expectedContentType, contentType)
		}
	case <-ctx.Done():
		// Context was canceled unexpectedly. Report the same.
		t.Fatalf("Context canceled when waiting for start container response: %v", ctx.Err())
	}
}

func TestStartContainerNotFound(t *testing.T) {
	t.Parallel()
	client := newTestClient(&FakeRoundTripper{message: "no such container", status: http.StatusNotFound})
	err := client.StartContainer("a2344", &HostConfig{})
	expectNoSuchContainer(t, "a2344", err)
}

func TestStartContainerAlreadyRunning(t *testing.T) {
	t.Parallel()
	client := newTestClient(&FakeRoundTripper{message: "container already running", status: http.StatusNotModified})
	err := client.StartContainer("a2334", &HostConfig{})
	expected := &ContainerAlreadyRunning{ID: "a2334"}
	if !reflect.DeepEqual(err, expected) {
		t.Errorf("StartContainer: Wrong error returned. Want %#v. Got %#v.", expected, err)
	}
}

func TestStartContainerWhenContextTimesOut(t *testing.T) {
	t.Parallel()
	rt := sleepyRoudTripper{sleepDuration: 200 * time.Millisecond}

	client := newTestClient(&rt)

	ctx, cancel := context.WithTimeout(context.TODO(), 100*time.Millisecond)
	defer cancel()

	err := client.StartContainerWithContext("id", nil, ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Expected 'DeadlineExceededError', got: %v", err)
	}
}
