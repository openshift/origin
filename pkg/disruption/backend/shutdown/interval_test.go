package shutdown

import (
	"io"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/openshift/origin/pkg/disruption/backend"
	"github.com/openshift/origin/pkg/disruption/sampler"

	"github.com/google/go-cmp/cmp"
)

func TestShutdownIntervalTracker(t *testing.T) {
	newShutdownResponse := func(shutdown bool, elapsed time.Duration, host string) *backend.ShutdownResponse {
		return &backend.ShutdownResponse{
			ShutdownInProgress:    shutdown,
			ShutdownDelayDuration: time.Minute,
			Elapsed:               elapsed,
			Hostname:              host,
		}
	}
	newSample := func(id uint64, at time.Time, err error, reused bool, sr *backend.ShutdownResponse, resp *http.Response) backend.SampleResult {
		if at.IsZero() {
			at = time.Now()
		}
		return backend.SampleResult{
			Sample: &sampler.Sample{ID: 1, StartedAt: at, Err: err},
			RequestResponse: backend.RequestResponse{
				Response: resp,
				RequestContextAssociatedData: backend.RequestContextAssociatedData{
					ShutdownResponse: sr,
					GotConnInfo: &backend.GotConnInfo{
						Reused: reused,
					},
				},
			},
		}
	}

	type wantHave struct {
		have []backend.SampleResult
		want *shutdownInterval
	}

	tests := []struct {
		name       string
		wantHaveFn func() wantHave
	}{
		{
			name: "shutdown sequence in progress, ends with the same host",
			wantHaveFn: func() wantHave {
				now := time.Now()
				samples := []backend.SampleResult{
					newSample(1, now.Add(-2*time.Second), nil, true, newShutdownResponse(false, 0, "foo"), nil),
					newSample(2, now.Add(-time.Second), nil, false, newShutdownResponse(false, 0, "foo"), nil),
					// shutdown in progress
					newSample(3, now, nil, true, newShutdownResponse(true, 1*time.Second, "foo"), nil),
					newSample(4, now.Add(3*time.Second), nil, false, newShutdownResponse(true, 1*time.Second, "foo"), nil),
					newSample(5, now.Add(4*time.Second), nil, true, newShutdownResponse(true, 7*time.Second, "foo"), retryAfter()),
					newSample(6, now.Add(5*time.Second), nil, false, newShutdownResponse(true, 7*time.Second, "foo"), retryAfter()),
					newSample(7, now.Add(6*time.Second), nil, true, newShutdownResponse(true, 5*time.Second, "foo"), nil),
					newSample(8, now.Add(7*time.Second), nil, false, newShutdownResponse(true, 5*time.Second, "foo"), nil),
					// shutdown sequence ends with a response from the same host
					newSample(9, now.Add(-time.Second), nil, false, newShutdownResponse(false, 0, "foo"), nil),
				}

				from := now.Add(-time.Second)
				interval := &shutdownInterval{
					Host:                          "foo",
					From:                          from,
					To:                            from.Add(time.Minute + 15*time.Second),
					DelayDuration:                 time.Minute,
					MaxElapsedWithNewConnection:   7 * time.Second,
					MaxElapsedWithConnectionReuse: 7 * time.Second,
					// Sample 5 and 6 have retry-after, they should be added to failures
					Failures: samples[4:6],
				}
				return wantHave{have: samples, want: interval}
			},
		},
		{
			name: "shutdown sequence in progress, ends with a different host",
			wantHaveFn: func() wantHave {
				now := time.Now()
				from := now.Add(-time.Second) // from sample 3
				to := from.Add(time.Minute + 15*time.Second)
				samples := []backend.SampleResult{
					newSample(1, now.Add(-2*time.Second), nil, true, newShutdownResponse(false, 0, "foo"), nil),
					newSample(2, now.Add(-time.Second), nil, false, newShutdownResponse(false, 0, "foo"), nil),
					// shutdown in progress
					newSample(3, now, nil, true, newShutdownResponse(true, 1*time.Second, "foo"), nil),
					newSample(4, now.Add(3*time.Second), nil, false, newShutdownResponse(true, 1*time.Second, "foo"), nil),
					newSample(5, now.Add(4*time.Second), nil, true, newShutdownResponse(true, 7*time.Second, "foo"), retryAfter()),
					newSample(6, now.Add(5*time.Second), io.EOF, false, newShutdownResponse(true, 7*time.Second, "foo"), nil),
					newSample(7, now.Add(6*time.Second), nil, true, newShutdownResponse(true, 5*time.Second, "foo"), nil),
					newSample(8, now.Add(7*time.Second), nil, false, newShutdownResponse(true, 5*time.Second, "foo"), nil),
					// shutdown sequence ends with a response from a different
					// host that is received after the interval ends.
					newSample(9, to.Add(time.Second), nil, false, newShutdownResponse(false, 0, "bar"), nil),
				}
				interval := &shutdownInterval{
					Host:                          "foo",
					From:                          from,
					To:                            to,
					DelayDuration:                 time.Minute,
					MaxElapsedWithNewConnection:   7 * time.Second,
					MaxElapsedWithConnectionReuse: 7 * time.Second,
					// Sample 5 is a retry-after, and 6 is an error
					Failures: samples[4:6],
				}
				return wantHave{have: samples, want: interval}
			},
		},
		{
			name: "shutdown sequence in progress, disruption test ends",
			wantHaveFn: func() wantHave {
				now := time.Now()
				from := now.Add(-time.Second) // from sample 3
				to := from.Add(time.Minute + 15*time.Second)
				samples := []backend.SampleResult{
					newSample(1, now.Add(-2*time.Second), nil, true, newShutdownResponse(false, 0, "foo"), nil),
					newSample(2, now.Add(-time.Second), nil, false, newShutdownResponse(false, 0, "foo"), nil),
					// shutdown in progress
					newSample(3, now, nil, true, newShutdownResponse(true, 1*time.Second, "foo"), nil),
					newSample(4, now.Add(3*time.Second), nil, false, newShutdownResponse(true, 1*time.Second, "foo"), nil),
					newSample(5, now.Add(4*time.Second), nil, true, newShutdownResponse(true, 7*time.Second, "foo"), retryAfter()),
					newSample(6, now.Add(5*time.Second), io.EOF, false, newShutdownResponse(true, 7*time.Second, "foo"), nil),
					newSample(7, now.Add(6*time.Second), nil, true, newShutdownResponse(true, 5*time.Second, "foo"), nil),
					newSample(8, now.Add(7*time.Second), nil, false, newShutdownResponse(true, 5*time.Second, "foo"), nil),
					// no more samples arriving
					{},
				}
				interval := &shutdownInterval{
					Host:                          "foo",
					From:                          from,
					To:                            to,
					DelayDuration:                 time.Minute,
					MaxElapsedWithNewConnection:   7 * time.Second,
					MaxElapsedWithConnectionReuse: 7 * time.Second,
					// Sample 5 is a retry-after, and 6 is an error
					Failures: samples[4:6],
				}
				return wantHave{have: samples, want: interval}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler := &fakeHandler{}
			tracker := &sharedShutdownIntervalTracker{
				shutdownIntervalTracker: &shutdownIntervalTracker{
					handler:   handler,
					intervals: make(map[string]*shutdownInterval),
				},
			}

			wantHave := test.wantHaveFn()
			for i := range wantHave.have {
				tracker.Collect(wantHave.have[i])
			}

			if !reflect.DeepEqual(wantHave.want, handler.got) {
				t.Errorf("unexpected shutdown interval: %s", cmp.Diff(wantHave.want, handler.got))
			}
		})
	}
}

type fakeHandler struct {
	got *shutdownInterval
}

func (f *fakeHandler) Handle(si *shutdownInterval) {
	f.got = si
}

func retryAfter() *http.Response {
	return &http.Response{
		StatusCode: http.StatusTooManyRequests,
		Header:     http.Header{"Retry-After": []string{"1"}},
	}
}
