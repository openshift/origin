package docker

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"testing"
	"time"
)

func TestWaitContainer(t *testing.T) {
	t.Parallel()
	fakeRT := &FakeRoundTripper{message: `{"StatusCode": 56}`, status: http.StatusOK}
	client := newTestClient(fakeRT)
	id := "4fa6e0f0c6786287e131c3852c58a2e01cc697a68231826813597e4994f1d6e2"
	status, err := client.WaitContainer(id)
	if err != nil {
		t.Fatal(err)
	}
	if status != 56 {
		t.Errorf("WaitContainer(%q): wrong return. Want 56. Got %d.", id, status)
	}
	req := fakeRT.requests[0]
	if req.Method != http.MethodPost {
		t.Errorf("WaitContainer(%q): wrong HTTP method. Want %q. Got %q.", id, http.MethodPost, req.Method)
	}
	expectedURL, _ := url.Parse(client.getURL("/containers/" + id + "/wait"))
	if gotPath := req.URL.Path; gotPath != expectedURL.Path {
		t.Errorf("WaitContainer(%q): Wrong path in request. Want %q. Got %q.", id, expectedURL.Path, gotPath)
	}
}

func TestWaitContainerWithContext(t *testing.T) {
	t.Parallel()
	fakeRT := &FakeRoundTripper{message: `{"StatusCode": 56}`, status: http.StatusOK}
	client := newTestClient(fakeRT)
	id := "4fa6e0f0c6786287e131c3852c58a2e01cc697a68231826813597e4994f1d6e2"

	ctx, cancel := context.WithTimeout(context.TODO(), 1*time.Second)
	defer cancel()

	var status int
	waitError := make(chan error)
	go func() {
		var err error
		status, err = client.WaitContainerWithContext(id, ctx)
		waitError <- err
	}()
	select {
	case err := <-waitError:
		if err != nil {
			t.Fatal(err)
		}
		if status != 56 {
			t.Errorf("WaitContainer(%q): wrong return. Want 56. Got %d.", id, status)
		}
		req := fakeRT.requests[0]
		if req.Method != http.MethodPost {
			t.Errorf("WaitContainer(%q): wrong HTTP method. Want %q. Got %q.", id, http.MethodPost, req.Method)
		}
		expectedURL, _ := url.Parse(client.getURL("/containers/" + id + "/wait"))
		if gotPath := req.URL.Path; gotPath != expectedURL.Path {
			t.Errorf("WaitContainer(%q): Wrong path in request. Want %q. Got %q.", id, expectedURL.Path, gotPath)
		}
	case <-ctx.Done():
		// Context was canceled unexpectedly. Report the same.
		t.Fatalf("Context canceled when waiting for wait container response: %v", ctx.Err())
	}
}

func TestWaitContainerNotFound(t *testing.T) {
	t.Parallel()
	client := newTestClient(&FakeRoundTripper{message: "no such container", status: http.StatusNotFound})
	_, err := client.WaitContainer("a2334")
	expectNoSuchContainer(t, "a2334", err)
}

func TestWaitContainerWhenContextTimesOut(t *testing.T) {
	t.Parallel()
	rt := sleepyRoudTripper{sleepDuration: 200 * time.Millisecond}

	client := newTestClient(&rt)

	ctx, cancel := context.WithTimeout(context.TODO(), 100*time.Millisecond)
	defer cancel()

	_, err := client.WaitContainerWithContext("id", ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Expected 'DeadlineExceededError', got: %v", err)
	}
}
