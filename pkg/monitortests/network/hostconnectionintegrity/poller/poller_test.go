package poller

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
)

type fakeProber struct{ results []Result }

func (f *fakeProber) Probe(context.Context) Result {
	r := f.results[0]
	if len(f.results) > 1 {
		f.results = f.results[1:]
	}
	return r
}

var (
	ok       = Result{Reused: true}
	okNew    = Result{}
	timeout  = Result{Reused: true, Err: context.DeadlineExceeded, LocalAddr: "10.0.0.4:51700"}
	reset    = Result{Reused: true, Err: errors.New("read tcp 10.0.0.4:51700->10.0.0.2:6443: read: connection reset by peer")}
	newFail  = Result{Err: errors.New("dial tcp 10.0.0.2:6443: i/o timeout")}
	freshBad = Result{Err: errors.New("dial tcp 10.0.0.2:6443: connect: connection refused")}
)

func TestClassify(t *testing.T) {
	tests := []struct {
		name        string
		established Result
		fresh       Result
		want        FailureReason
	}{
		{"stall on established, fresh ok", timeout, okNew, ReasonStalled},
		{"reset on established, fresh ok", reset, okNew, ReasonReset},
		{"established fails, fresh fails", timeout, freshBad, ReasonOutage},
		{"reset but fresh fails", reset, freshBad, ReasonOutage},
		{"no reuse, fresh ok", newFail, okNew, ReasonNewConnectionFailed},
		{"no reuse, fresh fails", newFail, freshBad, ReasonOutage},
		{"reused conn failed, transparent retry ok", Result{Reused: true, RetriedAfterReusedFailure: true}, okNew, ReasonReset},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Classify(tt.established, tt.fresh); got != tt.want {
				t.Errorf("Classify() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWatcherEpisodes(t *testing.T) {
	base := time.Date(2026, 9, 17, 6, 25, 57, 0, time.UTC)
	now := base
	var emitted []monitorapi.Interval
	w := &Watcher{
		NodeName:    "master-1",
		Target:      Target{Backend: BackendAPIIntSelf, Name: "api-int", URL: "https://api-int:6443/readyz"},
		Established: &fakeProber{results: []Result{ok, timeout, timeout, ok, reset, ok}},
		Fresh:       &fakeProber{results: []Result{okNew}},
		Now:         func() time.Time { return now },
		Emit:        func(i monitorapi.Interval) { emitted = append(emitted, i) },
	}
	for i := 0; i < 6; i++ {
		w.Step(context.Background())
		now = now.Add(500 * time.Millisecond)
	}
	w.Flush()

	if len(emitted) != 2 {
		t.Fatalf("expected 2 episodes, got %d: %v", len(emitted), emitted)
	}
	if got := emitted[0].Message.Reason; got != monitorapi.IntervalReason(ReasonStalled) {
		t.Errorf("first episode reason = %v", got)
	}
	// the stall began after the last successful probe (step 0), and ended at step 3.
	if !emitted[0].From.Equal(base) || !emitted[0].To.Equal(base.Add(1500*time.Millisecond)) {
		t.Errorf("first episode %v - %v", emitted[0].From, emitted[0].To)
	}
	if emitted[0].Message.Annotations[AnnotationLocalAddr] != "10.0.0.4:51700" {
		t.Errorf("local addr annotation missing: %v", emitted[0].Message.Annotations)
	}
	if got := emitted[1].Message.Reason; got != monitorapi.IntervalReason(ReasonReset) {
		t.Errorf("second episode reason = %v", got)
	}
	if emitted[0].Locator.Keys[monitorapi.LocatorNodeKey] != "master-1" || emitted[0].Locator.Keys[LocatorBackendKey] != BackendAPIIntSelf {
		t.Errorf("unexpected locator %v", emitted[0].Locator)
	}
}

// TestHTTPProberAgainstRealServers checks that real reset and stall behaviour of an established connection
// is classified correctly.
func TestHTTPProberAgainstRealServers(t *testing.T) {
	var mode atomic.Int32 // 0 = answer, 1 = reset, 2 = hang
	release := make(chan struct{})
	srv := httptest.NewTLSServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		m := mode.Load()
		if m == 4 {
			// send headers, then reset in the middle of the body.
			mode.Store(0)
			rw.Header().Set("Content-Length", "1000")
			rw.WriteHeader(http.StatusUnauthorized)
			if n, err := rw.Write([]byte("partial")); err != nil || n != len("partial") {
				t.Errorf("partial body write: wrote %d bytes, err %v", n, err)
				return
			}
			rw.(http.Flusher).Flush()
			hj, _ := rw.(http.Hijacker)
			conn, _, err := hj.Hijack()
			if err != nil {
				return
			}
			if tlsConn, ok := conn.(interface{ NetConn() net.Conn }); ok {
				if tcp, ok := tlsConn.NetConn().(*net.TCPConn); ok {
					_ = tcp.SetLinger(0)
				}
				_ = tlsConn.NetConn().Close()
				return
			}
			_ = conn.Close()
			return
		}
		if m == 3 {
			// reset exactly one request, answer the transparent retry.
			mode.Store(0)
			m = 1
		}
		switch m {
		case 1:
			hj, _ := rw.(http.Hijacker)
			conn, _, err := hj.Hijack()
			if err != nil {
				return
			}
			if tcp, ok := conn.(*net.TCPConn); ok {
				_ = tcp.SetLinger(0)
			} else if tlsConn, ok := conn.(interface{ NetConn() net.Conn }); ok {
				if tcp, ok := tlsConn.NetConn().(*net.TCPConn); ok {
					_ = tcp.SetLinger(0)
				}
				_ = tlsConn.NetConn().Close()
				return
			}
			_ = conn.Close()
		case 2:
			select {
			case <-release:
			case <-r.Context().Done():
			}
		default:
			rw.WriteHeader(http.StatusUnauthorized)
		}
	}))
	defer srv.Close()
	defer close(release)

	established := NewHTTPProber(srv.URL, true, 300*time.Millisecond)
	fresh := NewHTTPProber(srv.URL, false, 300*time.Millisecond)
	ctx := context.Background()

	if r := established.Probe(ctx); r.Err != nil {
		t.Fatalf("first probe failed: %v", r.Err)
	}
	if r := established.Probe(ctx); r.Err != nil || !r.Reused {
		t.Fatalf("expected a reused connection, got %+v", r)
	}

	mode.Store(2)
	stalled := established.Probe(ctx)
	mode.Store(0)
	if stalled.Err == nil || !IsTimeout(stalled.Err) {
		t.Fatalf("expected timeout, got %+v", stalled)
	}
	if got := Classify(stalled, fresh.Probe(ctx)); got != ReasonStalled {
		t.Errorf("stall classified as %v", got)
	}

	// re-establish, then reset it.
	_ = established.Probe(ctx)
	if r := established.Probe(ctx); !r.Reused {
		t.Fatalf("expected reuse before reset test")
	}
	mode.Store(1)
	resetRes := established.Probe(ctx)
	mode.Store(0)
	if resetRes.Err == nil {
		t.Fatalf("expected error on reset")
	}
	if got := Classify(resetRes, fresh.Probe(ctx)); got != ReasonReset {
		t.Errorf("reset classified as %v (err %v)", got, resetRes.Err)
	}

	// a reset that net/http hides by transparently retrying on a new connection must still be reported.
	_ = established.Probe(ctx)
	if r := established.Probe(ctx); !r.Reused {
		t.Fatalf("expected reuse before hidden reset test")
	}
	mode.Store(3)
	hidden := established.Probe(ctx)
	if !hidden.RetriedAfterReusedFailure {
		t.Fatalf("expected transparent retry to be detected, got %+v", hidden)
	}
	if got := Classify(hidden, fresh.Probe(ctx)); got != ReasonReset {
		t.Errorf("hidden reset classified as %v", got)
	}

	// a reset after the headers arrived must not count as a success.
	_ = established.Probe(ctx)
	if r := established.Probe(ctx); !r.Reused {
		t.Fatalf("expected reuse before body reset test")
	}
	mode.Store(4)
	bodyReset := established.Probe(ctx)
	if bodyReset.Err == nil {
		t.Fatalf("expected an error when the body is cut off, got %+v", bodyReset)
	}
	if got := Classify(bodyReset, fresh.Probe(ctx)); got != ReasonReset {
		t.Errorf("body reset classified as %v (err %v)", got, bodyReset.Err)
	}
}

func TestWatcherReasonChangeDoesNotOverlap(t *testing.T) {
	base := time.Date(2026, 9, 17, 6, 25, 57, 0, time.UTC)
	now := base
	var emitted []monitorapi.Interval
	w := &Watcher{
		NodeName:    "master-1",
		Target:      Target{Backend: BackendPeerKubelet, Name: "master-0"},
		Established: &fakeProber{results: []Result{ok, timeout, reset, ok}},
		Fresh:       &fakeProber{results: []Result{okNew}},
		Now:         func() time.Time { return now },
		Emit:        func(i monitorapi.Interval) { emitted = append(emitted, i) },
	}
	for i := 0; i < 4; i++ {
		w.Step(context.Background())
		now = now.Add(500 * time.Millisecond)
	}
	if len(emitted) != 2 {
		t.Fatalf("expected Stalled and Reset episodes, got %d", len(emitted))
	}
	if emitted[1].From.Before(emitted[0].To) {
		t.Errorf("episodes overlap: %v-%v and %v-%v", emitted[0].From, emitted[0].To, emitted[1].From, emitted[1].To)
	}
}

func TestWatcherLifecycleIntervalsOnHealthyRun(t *testing.T) {
	now := time.Date(2026, 9, 17, 6, 0, 0, 0, time.UTC)
	var emitted []monitorapi.Interval
	w := &Watcher{
		NodeName:    "worker-a",
		Target:      Target{Backend: BackendPeerKubelet, Name: "master-0"},
		Established: &fakeProber{results: []Result{ok}},
		Fresh:       &fakeProber{results: []Result{okNew}},
		Now:         func() time.Time { return now },
		Emit:        func(i monitorapi.Interval) { emitted = append(emitted, i) },
	}
	w.Started()
	for i := 0; i < 5; i++ {
		w.Step(context.Background())
	}
	w.Stopped()
	if len(emitted) != 2 {
		t.Fatalf("expected start and stop intervals only, got %d", len(emitted))
	}
	if emitted[0].Message.Reason != monitorapi.IntervalReason(ReasonWatchStarted) || emitted[1].Message.Reason != monitorapi.IntervalReason(ReasonWatchStopped) {
		t.Errorf("unexpected lifecycle reasons %v %v", emitted[0].Message.Reason, emitted[1].Message.Reason)
	}
	if emitted[0].Level != monitorapi.Info {
		t.Errorf("lifecycle intervals must be Info")
	}
}

func TestRingPeers(t *testing.T) {
	for _, n := range []int{1, 2, 3, 100} {
		peers := map[string]string{}
		for i := 0; i < n; i++ {
			peers[fmt.Sprintf("node-%03d", i)] = fmt.Sprintf("10.0.%d.%d", i/250, i%250+1)
		}
		probedBy := map[string]int{}
		for name := range peers {
			got := RingPeers(name, peers, 3)
			want := n - 1
			if want > 3 {
				want = 3
			}
			if len(got) != want {
				t.Errorf("n=%d %s: expected %d peers, got %v", n, name, want, got)
			}
			for _, p := range got {
				if p == name {
					t.Errorf("n=%d: %s probes itself", n, name)
				}
				probedBy[p]++
			}
		}
		if n > 1 {
			for name := range peers {
				if probedBy[name] == 0 {
					t.Errorf("n=%d: %s is not probed by anyone", n, name)
				}
			}
		}
	}
}

func TestBuildTargets(t *testing.T) {
	peers, err := ParsePeers([]string{"master-0=10.0.0.5", "master-1=10.0.0.4", "worker-a=10.0.128.2"})
	if err != nil {
		t.Fatal(err)
	}
	got := BuildTargets("master-1", peers, 3, []string{"master-0", "master-1"}, "https://api-int.example:6443")
	counts := map[string]int{}
	for _, tt := range got {
		counts[tt.Backend]++
		if tt.Backend == BackendPeerKubelet && tt.Name == "master-1" {
			t.Errorf("poller must not probe itself")
		}
	}
	if counts[BackendPeerKubelet] != 2 || counts[BackendAPIIntSelf] != 1 || counts[BackendLocalhostAPIServer] != 1 {
		t.Errorf("unexpected targets %v", got)
	}
	got = BuildTargets("worker-a", peers, 3, []string{"master-0", "master-1"}, "")
	for _, tt := range got {
		if tt.Backend != BackendPeerKubelet {
			t.Errorf("worker without api-int should only probe peers, got %v", tt)
		}
	}
	if _, err := ParsePeers([]string{"bad"}); err == nil {
		t.Errorf("expected error for invalid peer")
	}
}

func TestWatcherCancelledProbeIsNotAnOutage(t *testing.T) {
	now := time.Date(2026, 9, 17, 6, 0, 0, 0, time.UTC)
	var emitted []monitorapi.Interval
	cancelled := Result{Reused: true, Err: context.Canceled}
	w := &Watcher{
		NodeName:    "worker-a",
		Target:      Target{Backend: BackendPeerKubelet, Name: "master-0"},
		Established: &fakeProber{results: []Result{ok, cancelled}},
		Fresh:       &fakeProber{results: []Result{{Err: context.Canceled}}},
		Now:         func() time.Time { return now },
		Emit:        func(i monitorapi.Interval) { emitted = append(emitted, i) },
	}
	ctx, cancel := context.WithCancel(context.Background())
	w.Step(ctx)
	cancel()
	w.Step(ctx)
	w.Stopped()
	for _, i := range emitted {
		if i.Message.Reason != monitorapi.IntervalReason(ReasonWatchStopped) {
			t.Errorf("stopping the poller must not record an episode, got %s", i.Message.Reason)
		}
	}
}

func TestValidate(t *testing.T) {
	valid := func() *Options {
		return &Options{MyNodeName: "worker-a", Namespace: "ns", MaxPeers: 3, Peers: []string{"worker-a=10.0.0.1", "worker-b=fd00::2"}}
	}
	if err := valid().Validate(); err != nil {
		t.Fatalf("valid options rejected: %v", err)
	}
	for name, mutate := range map[string]func(*Options){
		"missing node name": func(o *Options) { o.MyNodeName = "" },
		"missing namespace": func(o *Options) { o.Namespace = "" },
		"zero max peers":    func(o *Options) { o.MaxPeers = 0 },
		"peer without IP":   func(o *Options) { o.Peers = []string{"worker-a"} },
		"peer with bad IP":  func(o *Options) { o.Peers = []string{"worker-a=not-an-ip"} },
		"peer without name": func(o *Options) { o.Peers = []string{"=10.0.0.1"} },
	} {
		o := valid()
		mutate(o)
		if err := o.Validate(); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}
