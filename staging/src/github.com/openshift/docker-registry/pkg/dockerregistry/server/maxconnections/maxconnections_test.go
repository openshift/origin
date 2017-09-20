package maxconnections

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestMaxConnections(t *testing.T) {
	const timeout = 1 * time.Second

	maxRunning := 1
	maxInQueue := 2
	maxWaitInQueue := time.Duration(0) // disable timeout
	lim := NewLimiter(maxRunning, maxInQueue, maxWaitInQueue)

	handlerBarrier := make(chan struct{}, maxRunning+maxInQueue+1)
	h := New(lim, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-handlerBarrier
		http.Error(w, "OK", http.StatusOK)
	}))

	ts := httptest.NewServer(h)
	defer ts.Close()
	defer func() {
		// Finish all pending requests in case of an error.
		// This prevents ts.Close() from being stuck.
		close(handlerBarrier)
	}()

	c := newCounter()
	done := make(chan struct{})
	wait := func(reason string) {
		select {
		case <-done:
		case <-time.After(timeout):
			t.Fatal(reason)
		}
	}
	for i := 0; i < maxRunning+maxInQueue+1; i++ {
		go func() {
			res, err := http.Get(ts.URL)
			if err != nil {
				t.Errorf("failed to get %s: %s", ts.URL, err)
			}
			c.Add(res.StatusCode, 1)
			done <- struct{}{}
		}()
	}

	wait("timeout while waiting one failed client")

	// expected state: 1 running, 2 in queue, 1 failed
	if expected := (countM{429: 1}); !c.Equal(expected) {
		t.Errorf("c = %v, want %v", c.Values(), expected)
	}

	handlerBarrier <- struct{}{}
	wait("timeout while waiting the first succeed client")

	// expected state: 1 running, 1 in queue, 1 failed, 1 succeed
	if expected := (countM{200: 1, 429: 1}); !c.Equal(expected) {
		t.Errorf("c = %v, want %v", c.Values(), expected)
	}

	handlerBarrier <- struct{}{}
	wait("timeout while waiting the second succeed client")

	// expected state: 1 running, 0 in queue, 1 failed, 2 succeed
	if expected := (countM{200: 2, 429: 1}); !c.Equal(expected) {
		t.Errorf("c = %v, want %v", c.Values(), expected)
	}

	handlerBarrier <- struct{}{}
	wait("timeout while waiting the third succeed client")

	// expected state: 0 running, 0 in queue, 1 failed, 3 succeed
	if expected := (countM{200: 3, 429: 1}); !c.Equal(expected) {
		t.Errorf("c = %v, want %v", c.Values(), expected)
	}
}
