package shutdown

import (
	"fmt"
	"sync"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"

	"github.com/openshift/origin/pkg/disruption/backend"
	backendsampler "github.com/openshift/origin/pkg/disruption/backend/sampler"

	"k8s.io/client-go/tools/events"
	"k8s.io/kubernetes/test/e2e/framework"
)

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
// This function returns an instance of WantEventRecorderAndMonitorRecorder, the test
// driver can use it to pass along the shared event recorder and monitor.
func NewSharedShutdownIntervalTracker(delegate backendsampler.SampleCollector, descriptor backend.TestDescriptor,
	monitorRecorder monitorapi.RecorderWriter, eventRecorder events.EventRecorder) (backendsampler.SampleCollector, backend.WantEventRecorderAndMonitorRecorder) {
	handler := newCIShutdownIntervalHandler(descriptor, monitorRecorder, eventRecorder)
	return &sharedShutdownIntervalTracker{
		shutdownIntervalTracker: &shutdownIntervalTracker{
			delegate:  delegate,
			handler:   handler,
			intervals: make(map[string]*shutdownInterval),
		},
	}, handler
}

// allows a set of disruption test(s) to share the same shutdown
// interval tracker instance so we have one interval in CI
// for each kube-apiserver graceful shutdown window.
type sharedShutdownIntervalTracker struct {
	lock sync.Mutex
	*shutdownIntervalTracker
}

func (st *sharedShutdownIntervalTracker) Collect(bs backend.SampleResult) {
	// we receive sample in ordered sequence, 1, 2, ... n
	if st.delegate != nil {
		st.delegate.Collect(bs)
	}

	st.lock.Lock()
	defer st.lock.Unlock()
	st.collect(bs)
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

func (t *shutdownIntervalTracker) Collect(bs backend.SampleResult) {
	// we receive sample in ordered sequence, 1, 2, ... n
	if t.delegate != nil {
		t.delegate.Collect(bs)
	}
	t.collect(bs)
}

func (t *shutdownIntervalTracker) closeInterval(condFn func(*shutdownInterval) bool) {
	for key, interval := range t.intervals {
		if !condFn(interval) {
			continue
		}
		delete(t.intervals, key)
		t.handler.Handle(interval)
	}
}

func (t *shutdownIntervalTracker) collect(bs backend.SampleResult) {
	if bs.Sample == nil {
		t.closeInterval(func(_ *shutdownInterval) bool {
			// no more samples, so close all open shutdown intervals
			return true
		})
		return
	}
	sr := bs.ShutdownResponse
	if sr == nil {
		if bs.Sample.Err != nil {
			// we don't have any shutdown response header, so we can't
			// correlate this sample to any particular host, for now we
			// allocate this error to all open shutdown interval(s)
			for _, t := range t.intervals {
				t.UnknownHostFailures = append(t.UnknownHostFailures, bs)
			}
		}
		return
	}

	if sr.ShutdownInProgress {
		framework.Logf("DisruptionTest: shutdown response seen: %s", sr.String())
	}

	interval, ok := t.intervals[sr.Hostname]
	switch {
	case sr.ShutdownInProgress:
		if !ok {
			// we are seeing a new shutdown interval in progress from
			// this host, this is when we create a new interval.
			from := bs.Sample.StartedAt.Add(-sr.Elapsed)
			to := from.Add(sr.ShutdownDelayDuration + 15*time.Second)
			interval = &shutdownInterval{
				Host:          sr.Hostname,
				DelayDuration: sr.ShutdownDelayDuration,
				From:          from,
				To:            to,
			}
			t.intervals[sr.Hostname] = interval

			framework.Logf("DisruptionTest: new shutdown interval seen: %s", interval.String())
		}
		// keep track of maximum elapsed seconds since the shutdown started
		if bs.GotConnInfo != nil {
			switch {
			case bs.GotConnInfo.Reused:
				if sr.Elapsed > interval.MaxElapsedWithConnectionReuse {
					interval.MaxElapsedWithConnectionReuse = sr.Elapsed
				}
			default:
				if sr.Elapsed > interval.MaxElapsedWithNewConnection {
					interval.MaxElapsedWithNewConnection = sr.Elapsed
				}
			}
		}
		if _, retry := bs.IsRetryAfter(); retry || bs.Sample.Err != nil {
			interval.Failures = append(interval.Failures, bs)
		}
	case ok:
		// we have a record of an interval in progress for this host, we close
		// the interval since this host has restarted and call the handler.
		t.handler.Handle(interval)
		delete(t.intervals, sr.Hostname)
	default:
		t.closeInterval(func(s *shutdownInterval) bool {
			// we close any shutdown interval that has elapsed
			return bs.Sample.StartedAt.After(s.To)
		})
	}
}

// shutdownInterval holds contextual information related to a kube-apiserver
// graceful shutdown window, from the disruption test point of view.
// The summary of data is generated by inspecting the values from the
// shutdown response header 'X-Openshift-Disruption' from requests
// hitting the given apiserver while it was shutting down.
type shutdownInterval struct {
	// Host is the host of the apiserver process that is shutting down
	Host string

	// From is the calculated length of the shutdown interval, it is derived
	// from the time at which the first request arrived at this apiserver
	// instance while it was shutting down, this helps us define
	// the shutdown interval.
	From time.Time

	// To is the end of the shutdown interval, it is computed as below:
	//   From + DelayDuration + 15s
	To time.Time

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

	// Failures is the list of request(s) that hit this apiserver (while it
	// was shutting down) instance and failed.
	Failures []backend.SampleResult

	// UnknownHostFailures is the list of request(s) that coincided with the
	// given shutdown interval and returned error and the host name is not
	// known, these failed samples are put into this bucket.
	UnknownHostFailures []backend.SampleResult
}

func (s shutdownInterval) String() string {
	return fmt.Sprintf("host=%s shutdown-delay-duration=%s max-elapsed-reuse=%s max-elapsed-new=%s failure=%d",
		s.Host, s.DelayDuration.Round(time.Second), s.MaxElapsedWithConnectionReuse.Round(time.Second), s.MaxElapsedWithNewConnection.Round(time.Second), len(s.Failures))
}
