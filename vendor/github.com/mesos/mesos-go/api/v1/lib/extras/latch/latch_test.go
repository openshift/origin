package latch_test

import (
	"testing"

	. "github.com/mesos/mesos-go/api/v1/lib/extras/latch"
)

func TestInterface(t *testing.T) {
	l := New()
	if l == nil {
		t.Fatalf("expected a valid latch, not nil")
	}
	if l.Closed() {
		t.Fatalf("expected new latch to be non-closed")
	}
	select {
	case <-l.Done():
		t.Fatalf("Done chan unexpectedly closed for a new latch")
	default:
	}
	for i := 0; i < 2; i++ {
		l.Close() // multiple calls to close should not panic
	}
	if !l.Closed() {
		t.Fatalf("expected closed latch")
	}
	select {
	case <-l.Done():
	default:
		t.Fatalf("Done chan unexpectedly non-closed for a closed latch")
	}
}
