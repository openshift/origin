package util

import (
	"fmt"
	"net"
	"reflect"
	"testing"
)

func TestHostnameMatchSpecCandidates(t *testing.T) {
	testcases := []struct {
		Hostname      string
		ExpectedSpecs []string
	}{
		{
			Hostname:      "",
			ExpectedSpecs: nil,
		},
		{
			Hostname:      "a",
			ExpectedSpecs: []string{"a", "*"},
		},
		{
			Hostname:      "foo.bar",
			ExpectedSpecs: []string{"foo.bar", "*.bar", "*.*"},
		},
	}

	for _, tc := range testcases {
		specs := HostnameMatchSpecCandidates(tc.Hostname)
		if !reflect.DeepEqual(specs, tc.ExpectedSpecs) {
			t.Errorf("%s: Expected %#v, got %#v", tc.Hostname, tc.ExpectedSpecs, specs)
		}
	}
}

func TestHostnameMatches(t *testing.T) {
	testcases := []struct {
		Hostname      string
		Spec          string
		ExpectedMatch bool
	}{
		// Empty hostname matches nothing
		{Hostname: "", Spec: "", ExpectedMatch: false},

		// Empty spec matches nothing
		{Hostname: "a", Spec: "", ExpectedMatch: false},

		// Exact match
		{Hostname: "a", Spec: "a", ExpectedMatch: true},
		// Single segment wildcard match
		{Hostname: "a", Spec: "*", ExpectedMatch: true},

		// Mismatched segment count should not match
		{Hostname: "a", Spec: "*.a", ExpectedMatch: false},
		{Hostname: "a", Spec: "*.*", ExpectedMatch: false},

		// Exact match, multi-segment
		{Hostname: "a.b", Spec: "a.b", ExpectedMatch: true},
		// Wildcard subdomain match
		{Hostname: "a.b", Spec: "*.b", ExpectedMatch: true},
		// Multi-level wildcard match
		{Hostname: "a.b", Spec: "*.*", ExpectedMatch: true},

		// Only subdomain wildcards are allowed
		{Hostname: "a.b", Spec: "a.*", ExpectedMatch: false},
		// Mismatched segment count should not match
		{Hostname: "a.b", Spec: "*.a.b", ExpectedMatch: false},
	}

	for i, tc := range testcases {
		matches := HostnameMatches(tc.Hostname, tc.Spec)
		if matches != tc.ExpectedMatch {
			t.Errorf("%d: Expected match=%v, got %v (hostname=%s, specs=%v)", i, tc.ExpectedMatch, matches, tc.Hostname, tc.Spec)
		}
	}
}

type fakeListener struct {
	Called     int
	Conn       net.Conn
	Err        error
	Closed     bool
	ListenAddr net.Addr
}

func (l *fakeListener) Accept() (net.Conn, error) {
	l.Called++
	return l.Conn, l.Err
}
func (l *fakeListener) Close() error {
	l.Closed = true
	return l.Err
}
func (l *fakeListener) Addr() net.Addr {
	return l.ListenAddr
}

type waitListener struct {
	Called     int
	Conn       net.Conn
	Err        error
	Closed     bool
	ListenAddr net.Addr
	Wait       chan struct{}
}

func (l *waitListener) Accept() (net.Conn, error) {
	l.Called++
	<-l.Wait
	return l.Conn, l.Err
}
func (l *waitListener) Close() error {
	l.Closed = true
	close(l.Wait)
	return l.Err
}
func (l *waitListener) Addr() net.Addr {
	return l.ListenAddr
}

func TestMultiListener(t *testing.T) {
	err1 := fmt.Errorf("error1")
	err2 := fmt.Errorf("error2")
	l1 := &fakeListener{Err: err1}
	l2 := &fakeListener{Err: err2}
	l := NewMultiListener(l1, l2).(*multiListener)

	first, second := 0, 0
	for i := 0; i < 100; i++ {
		_, err := l.Accept()
		switch {
		case err == err1:
			first++
		case err == err2:
			second++
		default:
			t.Errorf("received an unexpected error: %v", err)
		}
	}
	if err := l.Close(); err != err1 {
		t.Errorf("unexpected error on close: %v", err)
	}
	// the last accept is never received
	first = l1.Called - first
	if first == 1 {
		first = 0
	}
	second = l2.Called - second
	if second == 1 {
		second = 0
	}
	if first != 0 || second != 0 {
		t.Errorf("did not receive expected call amounts %#v %#v, %d %d", l1, l2, first, second)
	}
}

// TestMultiListenerWait verifies that a listener that does not return until it is closed
// will properly block the Close method of MultiListener
func TestMultiListenerCloseWait(t *testing.T) {
	err1 := fmt.Errorf("error1")
	err2 := fmt.Errorf("error2")
	l1 := &fakeListener{Err: err1}
	l2 := &waitListener{Err: err2, Wait: make(chan struct{})}
	l := NewMultiListener(l1, l2).(*multiListener)

	first, second := 0, 0
	for i := 0; i < 100; i++ {
		_, err := l.Accept()
		switch {
		case err == err1:
			first++
		case err == err2:
			second++
		default:
			t.Errorf("received an unexpected error: %v", err)
		}
	}
	if err := l.Close(); err != err1 {
		t.Errorf("unexpected error on close: %v", err)
	}
	// the last accept is never received
	first = l1.Called - first
	if first == 1 {
		first = 0
	}
	if first != 0 || second != 0 {
		t.Errorf("did not receive expected call amounts %#v %#v, %d %d", l1, l2, first, second)
	}
}
