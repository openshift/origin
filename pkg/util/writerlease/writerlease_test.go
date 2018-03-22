package writerlease

import (
	"testing"
	"time"
)

func TestWaitForLeader(t *testing.T) {
	l := New(0, 0)
	defer func() {
		if len(l.queued) > 0 {
			t.Fatalf("queue was not empty on shutdown: %#v", l.queued)
		}
	}()
	ch := make(chan struct{})
	defer close(ch)
	go l.Run(ch)
	calls := make(chan struct{}, 1)

	l.Try("test", func() (bool, bool) {
		calls <- struct{}{}
		return true, false
	})
	if !l.Wait() {
		t.Errorf("should have elected leader: %#v", l)
	}
	if len(calls) != 1 {
		t.Errorf("incorrect number of calls: %d", len(calls))
	}
}

func TestBecomeLeaderAfterRetry(t *testing.T) {
	l := New(0, 0)
	defer func() {
		if len(l.queued) > 0 {
			t.Fatalf("queue was not empty on shutdown: %#v", l.queued)
		}
	}()
	ch := make(chan struct{})
	defer close(ch)
	go l.Run(ch)
	calls := make(chan struct{}, 10)
	i := 0
	l.Try("test", func() (bool, bool) {
		calls <- struct{}{}
		i++
		return true, i < 5
	})
	if !l.Wait() {
		t.Errorf("should have elected leader: %#v", l)
	}
	if len(calls) != 5 {
		t.Errorf("incorrect number of calls: %d", len(calls))
	}
}

func TestBecomeFollowerAfterRetry(t *testing.T) {
	l := New(0, 0)
	l.backoff.Steps = 0
	l.backoff.Duration = 0
	defer func() {
		if len(l.queued) > 0 {
			t.Fatalf("queue was not empty on shutdown: %#v", l.queued)
		}
	}()
	ch := make(chan struct{})
	defer close(ch)
	go l.Run(ch)
	calls := make(chan struct{}, 10)
	i := 0
	l.Try("test", func() (bool, bool) {
		calls <- struct{}{}
		i++
		return false, i < 5
	})
	if l.Wait() {
		t.Errorf("should have become follower: %#v", l)
	}
	if len(calls) != 5 {
		t.Errorf("incorrect number of calls: %d", len(calls))
	}
}

func TestRunOverlappingWork(t *testing.T) {
	l := New(0, 0)
	l.backoff.Steps = 0
	l.backoff.Duration = 0
	done := make(chan struct{})
	defer func() {
		<-done
		if len(l.queued) > 0 {
			t.Fatalf("queue was not empty on shutdown: %#v", l.queued)
		}
	}()

	go func() {
		t.Logf("processing first")
		l.work()
		t.Logf("processing second")
		l.work()
		t.Logf("processing done")
		close(done)
	}()

	first := make(chan struct{})
	l.Try("test", func() (bool, bool) {
		first <- struct{}{}
		t.Logf("waiting for second item to be added")
		first <- struct{}{}
		return true, false
	})
	<-first
	second := make(chan struct{}, 1)
	l.Try("test", func() (bool, bool) {
		second <- struct{}{}
		return true, false
	})
	t.Logf("second item added")
	<-first
	<-second
	<-done
}

func TestExtend(t *testing.T) {
	l := New(10*time.Millisecond, 0)
	l.nowFn = func() time.Time { return time.Unix(0, 0) }
	l.backoff.Steps = 0
	l.backoff.Duration = 2 * time.Millisecond
	defer func() {
		if len(l.queued) > 0 {
			t.Fatalf("queue was not empty on shutdown: %#v", l.queued)
		}
	}()
	ch := make(chan struct{})
	defer close(ch)
	calls := make(chan struct{})
	go l.Run(ch)

	l.Try("test", func() (bool, bool) {
		calls <- struct{}{}
		return false, false
	})
	l.Try("test2", func() (bool, bool) {
		calls <- struct{}{}
		return false, false
	})
	<-calls
	l.Extend("test2")

	l.Wait()
	for l.queue.Len() > 0 {
		time.Sleep(time.Millisecond)
	}
	state, expires, _ := l.leaseState()
	if state != Follower || expires != time.Unix(0, int64(10*time.Millisecond)) {
		t.Errorf("unexpected lease state: %v %#v", expires.UnixNano(), l)
	}
}
