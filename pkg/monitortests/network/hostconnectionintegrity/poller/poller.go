// Package poller implements the "openshift-tests disruption watch-established-connections" command.
//
// The poller runs in the host network namespace of every node. For each target it keeps one long-lived
// (keep-alive) connection open and probes over it every ProbeInterval. When a probe on an established
// (reused) connection fails, it immediately probes the same target over a brand-new connection to tell
// apart a problem with that one connection (Reset / Stalled) from a general outage of the target (Outage).
// Every failure episode is written to stdout as a one-line JSON interval, which the monitor test collects
// from the pod logs after the run.
package poller

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	monitorserialization "github.com/openshift/origin/pkg/monitor/serialization"
)

const (
	// IntervalSource is the source of intervals emitted by the poller.
	IntervalSource monitorapi.IntervalSource = "EstablishedConnection"

	// Backend names.
	BackendPeerKubelet        = "peer-kubelet"
	BackendAPIIntSelf         = "api-int-self"
	BackendLocalhostAPIServer = "localhost-apiserver"

	// Locator keys.
	LocatorBackendKey monitorapi.LocatorKey = "backend"
	LocatorTargetKey  monitorapi.LocatorKey = "target"

	// Annotation keys.
	AnnotationLocalAddr monitorapi.AnnotationKey = "local-addr"
	AnnotationError     monitorapi.AnnotationKey = "error"
	// AnnotationFailedProbeStart is the start time (RFC3339Nano) of the first failed probe of the episode.
	// Together with the interval From (the last successful probe) it brackets when the failure began.
	AnnotationFailedProbeStart monitorapi.AnnotationKey = "failed-probe-start"
)

// FailureReason classifies a failed probe on an established connection.
type FailureReason string

const (
	// ReasonReset means the established connection was reset (RST / EPIPE) while a new connection to the same
	// target worked.
	ReasonReset FailureReason = "Reset"
	// ReasonStalled means the established connection stopped passing traffic (probe timed out without an error
	// from the peer) while a new connection to the same target worked. This is the "silent black hole" case.
	ReasonStalled FailureReason = "Stalled"
	// ReasonOutage means both the established connection and a new connection to the target failed.
	ReasonOutage FailureReason = "Outage"
	// ReasonNewConnectionFailed means a probe that had to open a new connection (nothing to reuse) failed.
	ReasonNewConnectionFailed FailureReason = "NewConnectionFailed"
)

// Target is one endpoint probed by the poller.
type Target struct {
	Backend string
	// Name identifies the target in the locator (peer node name, "api-int", "localhost").
	Name string
	URL  string
}

// Result is the outcome of one probe.
type Result struct {
	Err error
	// Reused is true when the probe started on an already established connection.
	Reused bool
	// LocalAddr is the local address of the established connection the probe started on (or of the new
	// connection if nothing was reused).
	LocalAddr string
	// RetriedAfterReusedFailure is true when net/http transparently retried the request on a new connection
	// because the reused connection failed before a response arrived. The probe may still have succeeded;
	// the established connection nevertheless broke.
	RetriedAfterReusedFailure bool
}

// Prober performs one HTTP probe. Separate implementations exist for the established and the new connection.
type Prober interface {
	Probe(ctx context.Context) Result
}

// httpProber probes a URL. Any HTTP response (including 401/403) counts as success: the poller measures
// connectivity, not authorization.
type httpProber struct {
	url     string
	client  *http.Client
	timeout time.Duration
}

func newTransport(keepAlive bool) *http.Transport {
	t := &http.Transport{
		// The probes are unauthenticated connectivity checks against self-signed kubelet/apiserver endpoints.
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
		// Force HTTP/1.1: keeps exactly one TCP connection per transport and avoids HTTP/2 connection
		// health checks hiding or delaying the failure we want to observe.
		TLSNextProto:      map[string]func(string, *tls.Conn) http.RoundTripper{},
		DisableKeepAlives: !keepAlive,
		MaxConnsPerHost:   1,
		DialContext:       (&net.Dialer{Timeout: 2 * time.Second, KeepAlive: -1}).DialContext,
	}
	if keepAlive {
		t.MaxIdleConnsPerHost = 1
		t.IdleConnTimeout = 0
	}
	return t
}

// NewHTTPProber returns a prober. keepAlive=true reuses one connection across probes.
func NewHTTPProber(url string, keepAlive bool, timeout time.Duration) Prober {
	return &httpProber{
		url:     url,
		client:  &http.Client{Transport: newTransport(keepAlive)},
		timeout: timeout,
	}
}

func (p *httpProber) Probe(ctx context.Context) Result {
	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	res := Result{}
	gotConns := 0
	trace := &httptrace.ClientTrace{
		GotConn: func(info httptrace.GotConnInfo) {
			gotConns++
			if gotConns == 1 {
				res.Reused = info.Reused
				if info.Conn != nil {
					res.LocalAddr = info.Conn.LocalAddr().String()
				}
				return
			}
			if res.Reused {
				res.RetriedAfterReusedFailure = true
			}
		},
	}
	req, err := http.NewRequestWithContext(httptrace.WithClientTrace(ctx, trace), http.MethodGet, p.url, nil)
	if err != nil {
		res.Err = err
		return res
	}
	resp, err := p.client.Do(req)
	if err != nil {
		res.Err = err
		return res
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	return res
}

var errReusedConnectionFailed = errors.New("established connection failed; request succeeded only after net/http retried it on a new connection")

// IsReset reports whether err indicates the peer (or something in the path) reset the connection.
func IsReset(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.EPIPE) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "connection reset by peer") || strings.Contains(msg, "broken pipe") ||
		strings.HasSuffix(msg, ": EOF")
}

// IsTimeout reports whether err is a timeout without any signal from the peer.
func IsTimeout(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
}

// Classify decides the failure reason for a failed probe on the established prober, given the result of an
// immediate probe of the same target over a new connection.
func Classify(established Result, fresh Result) FailureReason {
	if established.RetriedAfterReusedFailure && fresh.Err == nil {
		return ReasonReset
	}
	if !established.Reused {
		// The established prober had no connection to reuse; this says nothing about established connections.
		if fresh.Err != nil {
			return ReasonOutage
		}
		return ReasonNewConnectionFailed
	}
	if fresh.Err != nil {
		return ReasonOutage
	}
	if IsReset(established.Err) {
		return ReasonReset
	}
	// Timeouts and anything else that is not an explicit reset while the target is reachable over a new
	// connection are treated as the connection being black-holed.
	return ReasonStalled
}

// episode tracks consecutive failed probes on one target.
type episode struct {
	reason      FailureReason
	from        time.Time
	failedStart time.Time
	localAddr   string
	err         string
}

// Watcher probes one target.
type Watcher struct {
	NodeName    string
	Target      Target
	Established Prober
	Fresh       Prober
	Interval    time.Duration
	Now         func() time.Time
	Emit        func(monitorapi.Interval)

	lastOK  time.Time
	current *episode
}

// Step performs one probe cycle. Exported for tests.
func (w *Watcher) Step(ctx context.Context) {
	start := w.Now()
	res := w.Established.Probe(ctx)
	if res.Err == nil && res.RetriedAfterReusedFailure {
		// The established connection broke but net/http recovered on a new connection, so the target is
		// reachable: this is a reset of the established connection.
		res.Err = errReusedConnectionFailed
	}
	if res.Err == nil {
		w.closeEpisode(start)
		w.lastOK = w.Now()
		return
	}
	fresh := w.Fresh.Probe(ctx)
	reason := Classify(res, fresh)
	if w.current != nil && w.current.reason == reason {
		return
	}
	w.closeEpisode(start)
	// The failure began at some point after the last successful probe on this target. Use that as the start
	// of the interval so consumers can correlate the onset with other events.
	from := w.lastOK
	if from.IsZero() {
		from = start
	}
	w.current = &episode{reason: reason, from: from, failedStart: start, localAddr: res.LocalAddr, err: res.Err.Error()}
}

func (w *Watcher) closeEpisode(to time.Time) {
	if w.current == nil {
		return
	}
	w.Emit(BuildInterval(w.NodeName, w.Target, w.current.reason, w.current.from, w.current.failedStart, to, w.current.localAddr, w.current.err))
	w.current = nil
}

// Flush emits any open episode.
func (w *Watcher) Flush() {
	w.closeEpisode(w.Now())
}

// Run probes until ctx is done.
func (w *Watcher) Run(ctx context.Context) {
	ticker := time.NewTicker(w.Interval)
	defer ticker.Stop()
	for {
		w.Step(ctx)
		select {
		case <-ctx.Done():
			w.Flush()
			return
		case <-ticker.C:
		}
	}
}

// BuildLocator returns the locator used for poller intervals.
func BuildLocator(nodeName string, target Target) monitorapi.Locator {
	return monitorapi.Locator{
		Type: monitorapi.LocatorTypeNode,
		Keys: map[monitorapi.LocatorKey]string{
			monitorapi.LocatorNodeKey: nodeName,
			LocatorBackendKey:         target.Backend,
			LocatorTargetKey:          target.Name,
		},
	}
}

// BuildInterval builds the interval for one failure episode.
func BuildInterval(nodeName string, target Target, reason FailureReason, from, failedStart, to time.Time, localAddr, errMsg string) monitorapi.Interval {
	level := monitorapi.Error
	if reason == ReasonNewConnectionFailed {
		level = monitorapi.Warning
	}
	return monitorapi.NewInterval(IntervalSource, level).
		Locator(BuildLocator(nodeName, target)).
		Message(monitorapi.NewMessage().
			Reason(monitorapi.IntervalReason(reason)).
			WithAnnotation(AnnotationLocalAddr, localAddr).
			WithAnnotation(AnnotationError, errMsg).
			WithAnnotation(AnnotationFailedProbeStart, failedStart.UTC().Format(time.RFC3339Nano)).
			HumanMessagef("established connection from node/%s to %s %s (%s) failed: %s", nodeName, target.Backend, target.Name, target.URL, reason)).
		Display().
		Build(from, to)
}

// JSONEmitter writes intervals as one-line JSON to out. Safe for concurrent use.
func JSONEmitter(out io.Writer) func(monitorapi.Interval) {
	var lock sync.Mutex
	return func(i monitorapi.Interval) {
		b, err := monitorserialization.IntervalToOneLineJSON(i)
		if err != nil {
			fmt.Fprintf(out, "failed to serialize interval: %v\n", err)
			return
		}
		lock.Lock()
		defer lock.Unlock()
		fmt.Fprintln(out, string(b))
	}
}
