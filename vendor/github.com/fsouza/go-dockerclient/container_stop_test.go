package docker

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"reflect"
	"testing"
	"time"
)

func TestStopContainer(t *testing.T) {
	t.Parallel()
	fakeRT := &FakeRoundTripper{message: "", status: http.StatusNoContent}
	client := newTestClient(fakeRT)
	id := "4fa6e0f0c6786287e131c3852c58a2e01cc697a68231826813597e4994f1d6e2"
	err := client.StopContainer(id, 10)
	if err != nil {
		t.Fatal(err)
	}
	req := fakeRT.requests[0]
	if req.Method != http.MethodPost {
		t.Errorf("StopContainer(%q, 10): wrong HTTP method. Want %q. Got %q.", id, http.MethodPost, req.Method)
	}
	expectedURL, _ := url.Parse(client.getURL("/containers/" + id + "/stop"))
	if gotPath := req.URL.Path; gotPath != expectedURL.Path {
		t.Errorf("StopContainer(%q, 10): Wrong path in request. Want %q. Got %q.", id, expectedURL.Path, gotPath)
	}
}

func TestStopContainerWithContext(t *testing.T) {
	t.Parallel()
	fakeRT := &FakeRoundTripper{message: "", status: http.StatusNoContent}
	client := newTestClient(fakeRT)
	id := "4fa6e0f0c6786287e131c3852c58a2e01cc697a68231826813597e4994f1d6e2"

	ctx, cancel := context.WithTimeout(context.TODO(), 1*time.Second)
	defer cancel()

	stopError := make(chan error)
	go func() {
		stopError <- client.StopContainerWithContext(id, 10, ctx)
	}()
	select {
	case err := <-stopError:
		if err != nil {
			t.Fatal(err)
		}
		req := fakeRT.requests[0]
		if req.Method != http.MethodPost {
			t.Errorf("StopContainer(%q, 10): wrong HTTP method. Want %q. Got %q.", id, http.MethodPost, req.Method)
		}
		expectedURL, _ := url.Parse(client.getURL("/containers/" + id + "/stop"))
		if gotPath := req.URL.Path; gotPath != expectedURL.Path {
			t.Errorf("StopContainer(%q, 10): Wrong path in request. Want %q. Got %q.", id, expectedURL.Path, gotPath)
		}
	case <-ctx.Done():
		// Context was canceled unexpectedly. Report the same.
		t.Fatalf("Context canceled when waiting for stop container response: %v", ctx.Err())
	}
}

func TestStopContainerNotFound(t *testing.T) {
	t.Parallel()
	client := newTestClient(&FakeRoundTripper{message: "no such container", status: http.StatusNotFound})
	err := client.StopContainer("a2334", 10)
	expectNoSuchContainer(t, "a2334", err)
}

func TestStopContainerNotRunning(t *testing.T) {
	t.Parallel()
	client := newTestClient(&FakeRoundTripper{message: "container not running", status: http.StatusNotModified})
	err := client.StopContainer("a2334", 10)
	expected := &ContainerNotRunning{ID: "a2334"}
	if !reflect.DeepEqual(err, expected) {
		t.Errorf("StopContainer: Wrong error returned. Want %#v. Got %#v.", expected, err)
	}
}

func TestStopContainerWhenContextTimesOut(t *testing.T) {
	t.Parallel()
	rt := sleepyRoudTripper{sleepDuration: 300 * time.Millisecond}

	client := newTestClient(&rt)

	ctx, cancel := context.WithTimeout(context.TODO(), 50*time.Millisecond)
	defer cancel()

	err := client.StopContainerWithContext("id", 10, ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Expected 'DeadlineExceededError', got: %v", err)
	}
}
