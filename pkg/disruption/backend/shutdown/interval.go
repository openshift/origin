package shutdown

import (
	"fmt"
	"sync"
	"time"

	"github.com/openshift/origin/pkg/disruption/backend"
	backendsampler "github.com/openshift/origin/pkg/disruption/backend/sampler"

	"k8s.io/client-go/tools/events"
)

// NewShutdownIntervalTracker returns a SampleCollector that does the following:
//
//   - it goes through each sample result, and constructs shutdown interval(s)
//     from the 'X-Openshift-Disruption' response header, and then
//
//   - it records each shutdown interval in CI
//
//     delegate: the next SampleCollector in the chain to be invoked
//     monitor: Monitor API to start and end an interval in CI
//     eventRecorder: to create events associated with the intervals
//     locator: the CI locator assigned to this disruption test
//     name: name of the disruption test
func NewShutdownIntervalTracker(delegate backendsampler.SampleCollector, monitor backend.Monitor,
	eventRecorder events.EventRecorder, locator, name string) *shutdownIntervalTracker {
	return &shutdownIntervalTracker{
		delegate:  delegate,
		handler:   newCIShutdownIntervalHandler(monitor, eventRecorder, locator, name),
		intervals: make(map[string]*shutdownInterval),
	}
}

// NewSharedShutdownIntervalTracker returns a SampleCollector
// that does the following:
//
//   - it goes through each sample result, and constructs shutdown interval(s)
//     from the 'X-Openshift-Disruption' response header, and then
//
//   - it records each shutdown interval in CI
//
//     delegate: the next SampleCollector in the chain to be invoked
//     monitor: Monitor API to start and end an interval in CI
//     eventRecorder: to create events associated with the intervals
//     locator: the CI locator assigned to this disruption test
//     name: name of the disruption test
//
// NOTE: NewSharedShutdownIntervalTracker allows a single
// shutdownIntervalTracker instance to be used by multiple backend sampler
// instances safely. In CI, we want a single interval for each graceful
// shutdown event of the kube-apiserver, and request(s) from different
// backend sampler(s) will potentially hit the kube-apiserver
// during a graceful shutdown window.
func NewSharedShutdownIntervalTracker(delegate backendsampler.SampleCollector, monitor backend.Monitor,
	eventRecorder events.EventRecorder, locator, name string) *sharedShutdownIntervalTracker {
	handler := newCIShutdownIntervalHandler(monitor, eventRecorder, locator, name)
	return &sharedShutdownIntervalTracker{
		shutdownIntervalTracker: &shutdownIntervalTracker{
			delegate:  delegate,
			handler:   handler,
			intervals: make(map[string]*shutdownInterval),
		},
	}
}

// allows a set of disruption test(s) to share the same shutdown
// interval tracker instance so we have one interval in CI
// for each kube-apiserver graceful shutdown window.
type sharedShutdownIntervalTracker struct {
	lock sync.Mutex
	*shutdownIntervalTracker
}

func (s *sharedShutdownIntervalTracker) Collect(bs backend.SampleResult) {
	// we receive sample in ordered sequence, 1, 2, ... n
	if s.delegate != nil {
		s.delegate.Collect(bs)
	}

	s.lock.Lock()
	defer s.lock.Unlock()
	s.collect(bs)
}

// shutdownIntervalHandler receives shutdown interval(s) and handles them,
// this is an internal interface and exists for unit test purposes only.
type shutdownIntervalHandler interface {
	Handle(*shutdownInterval)
}

type shutdownIntervalTracker struct {
	delegate backendsampler.SampleCollector
	handler  shutdownIntervalHandler

	intervals map[string]*shutdownInterval
}

func (r *shutdownIntervalTracker) Collect(bs backend.SampleResult) {
	// we receive sample in ordered sequence, 1, 2, ... n
	if r.delegate != nil {
		r.delegate.Collect(bs)
	}
	r.collect(bs)
}

func (r *shutdownIntervalTracker) close(bs backend.SampleResult, condFn func(*shutdownInterval) bool) {
	for key, interval := range r.intervals {
		delete(r.intervals, key)
		if !condFn(interval) {
			continue
		}
		if bs.Sample != nil {
			interval.LastSampleSeenAt = bs.Sample.StartedAt
		}
		r.handler.Handle(interval)
	}
}

func (r *shutdownIntervalTracker) collect(bs backend.SampleResult) {
	if bs.Sample == nil {
		r.close(bs, func(_ *shutdownInterval) bool {
			// no more samples, so close all open shutdown intervals
			return true
		})
		return
	}
	sr := bs.ShutdownResponse
	if sr == nil {
		if bs.Sample.Err != nil {
			for _, t := range r.intervals {
				t.Failures = append(t.Failures, bs)
			}
		}
		return
	}

	if sr.ShutdownInProgress {
		t, ok := r.intervals[sr.Hostname]
		if !ok {
			// we are seeing a new intervals in progress from this host
			// this is when we open a new intervals handler
			t = &shutdownInterval{
				Host:              sr.Hostname,
				DelayDuration:     sr.ShutdownDelayDuration,
				FirstSampleSeenAt: bs.Sample.StartedAt,
			}
			r.intervals[sr.Hostname] = t
		}
		// keep track of maximum elapsed seconds since the window started
		if bs.GotConnInfo != nil {
			switch {
			case bs.GotConnInfo.Reused:
				if sr.Elapsed > t.MaxElapsedWithConnectionReuse {
					t.MaxElapsedWithConnectionReuse = sr.Elapsed
				}
			default:
				if sr.Elapsed > t.MaxElapsedWithNewConnection {
					t.MaxElapsedWithNewConnection = sr.Elapsed
				}
			}
		}
		if bs.Sample.Err != nil {
			t.Failures = append(t.Failures, bs)
		} else {
			t.Success = append(t.Success, bs)
		}
		return
	}

	t, ok := r.intervals[sr.Hostname]
	if ok {
		// we have a record of intervals in progress for this host
		// we close the handler now since this host is come back
		t.LastSampleSeenAt = bs.Sample.StartedAt
		r.handler.Handle(t)
		delete(r.intervals, sr.Hostname)
		return
	}
	r.close(bs, func(s *shutdownInterval) bool {
		// we keep a shutdown window open with some grace period, considering
		// the possibility that a faulty load balancer can send late request
		return bs.Sample.StartedAt.Sub(s.FirstSampleSeenAt) > s.DelayDuration+30*time.Second
	})
}

// shutdownInterval holds contextual information related to a kube-apiserver
// graceful shutdown window, from the disruption test point of view.
// The summary of data is generated by inspecting the values from the
// shutdown response header 'X-Openshift-Disruption' from requests
// hitting the given apiserver while it was shutting down.
type shutdownInterval struct {
	// Host is the host of the apiserver process that is shutting down
	Host string

	// DelayDuration is the value of the shutdown-delay-duration server run
	// option, as advertised by the apiserver
	DelayDuration time.Duration

	// MaxElapsedWithNewConnection is the duration the disruption test has been
	// in contact with this apiserver since it has received a TERM signal.
	// This is the time interval starting from the time the TERM signal was
	// received and ending at the time the last request hit this apiserver
	// instance while it was shutting down. This is measured for requests
	// arriving on new tcp connection(s).
	// This can be used as an indicator of how long it takes a certain load
	// balancer to switch to a different host when the given apiserver
	// is shutting down.
	MaxElapsedWithNewConnection time.Duration

	// MaxElapsedWithConnectionReuse is the duration the disruption test has
	// been in contact with this apiserver since it has received a TERM signal.
	// This is the time interval starting from the time the TERM signal was
	// received and ending at the time the last request hit this apiserver
	// instance while it was shutting down. This is measured for requests
	// arriving on existing tcp connection(s).
	// This can be used as an indicator of how long it takes a certain load
	// balancer to switch to a different host when the given apiserver
	// is shutting down.
	MaxElapsedWithConnectionReuse time.Duration

	// FirstSampleSeenAt is the time at which the first request arrived
	// at this apiserver instance while it was shutting down, this helps
	// us define the shutdown interval.
	FirstSampleSeenAt time.Time

	// LastSampleSeenAt is the time at which the last request arrived
	// at this apiserver instance while it was shutting down, this helps
	// us define the shutdown interval.
	LastSampleSeenAt time.Time

	// Failures is the list of request(s) that hit this apiserver (while it
	// was shutting down) instance and failed.
	Failures []backend.SampleResult

	// Success is the list of request(s) that hit this apiserver (while it
	// was shutting down) instance and succeeded.
	Success []backend.SampleResult
}

func (s shutdownInterval) String() string {
	return fmt.Sprintf("host=%s shutdown-delay-duration=%s max-elapsed-reuse=%s max-elapsed-new=%s failure=%d success=%d",
		s.Host, s.DelayDuration.Round(time.Second), s.MaxElapsedWithConnectionReuse.Round(time.Second), s.MaxElapsedWithNewConnection.Round(time.Second), len(s.Failures), len(s.Success))
}
