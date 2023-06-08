package disruption

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/openshift/origin/pkg/disruption/backend"
	"github.com/openshift/origin/pkg/disruption/sampler"
)

func TestDisruptionTracker(t *testing.T) {
	tests := []struct {
		name                   string
		samples                []backend.SampleResult
		unavailable, available int
		intervals              []interval
	}{
		{
			name: "no samples",
			samples: []backend.SampleResult{
				{Sample: nil},
			},
		},
		{
			name: "one good sample, zero expected",
			samples: []backend.SampleResult{
				{Sample: &sampler.Sample{ID: 1}},
			},
			available: 1,
			intervals: []interval{
				{from: 1, to: 1},
			},
		},
		{
			name: "one good sample, with finalizer, zero expected",
			samples: []backend.SampleResult{
				{Sample: &sampler.Sample{ID: 1}},
				{Sample: nil},
			},
			available: 1,
			intervals: []interval{
				{from: 1, to: 1},
			},
		},
		{
			name: "one bad sample, one unavailable window expected",
			samples: []backend.SampleResult{
				{Sample: &sampler.Sample{ID: 1, Err: fmt.Errorf("error")}},
				{Sample: nil},
			},
			unavailable: 1,
			intervals: []interval{
				{from: 1, to: 1},
			},
		},
		{
			name: "one bad sample with no finalizer, no unavailable window expected",
			samples: []backend.SampleResult{
				{Sample: &sampler.Sample{ID: 1, Err: fmt.Errorf("error")}},
			},
		},
		{
			name: "multiple good samples only, no disruption, zero expected",
			samples: []backend.SampleResult{
				{Sample: &sampler.Sample{ID: 1}},
				{Sample: &sampler.Sample{ID: 2}},
				{Sample: &sampler.Sample{ID: 3}},
				{Sample: &sampler.Sample{ID: 4}},
				{Sample: nil},
			},
			available: 1,
			intervals: []interval{
				{from: 1, to: 1},
			},
		},
		{
			name: "multiple good samples with a finalizer, no disruption, zero expected",
			samples: []backend.SampleResult{
				{Sample: &sampler.Sample{ID: 1}},
				{Sample: &sampler.Sample{ID: 2}},
				{Sample: &sampler.Sample{ID: 3}},
				{Sample: &sampler.Sample{ID: 4}},
				{Sample: nil},
			},
			available: 1,
			intervals: []interval{
				{from: 1, to: 1},
			},
		},
		{
			name: "same failures, one handler expected",
			samples: []backend.SampleResult{
				{Sample: &sampler.Sample{ID: 1, Err: fmt.Errorf("error")}},
				{Sample: &sampler.Sample{ID: 2, Err: fmt.Errorf("error")}},
				{Sample: &sampler.Sample{ID: 3, Err: fmt.Errorf("error")}},
				{Sample: &sampler.Sample{ID: 4, Err: fmt.Errorf("error")}},
				{Sample: nil},
			},
			unavailable: 1,
			intervals: []interval{
				{from: 1, to: 4},
			},
		},
		{
			name: "unavailable, and then available",
			samples: []backend.SampleResult{
				{Sample: &sampler.Sample{ID: 1}},
				{Sample: &sampler.Sample{ID: 2, Err: fmt.Errorf("error")}},
				{Sample: &sampler.Sample{ID: 3, Err: fmt.Errorf("error")}},
				{Sample: &sampler.Sample{ID: 4}},
				{Sample: &sampler.Sample{ID: 5}},
				{Sample: &sampler.Sample{ID: 6, Err: fmt.Errorf("error")}},
				{Sample: &sampler.Sample{ID: 7, Err: fmt.Errorf("error")}},
				{Sample: &sampler.Sample{ID: 8}},
				{Sample: nil},
			},
			unavailable: 2,
			available:   3,
			intervals: []interval{
				{from: 1, to: 1},
				{from: 2, to: 4},
				{from: 4, to: 6},
				{from: 6, to: 8},
				{from: 8, to: 8},
			},
		},
		{
			name: "unavailable, and then available with single error",
			samples: []backend.SampleResult{
				{Sample: &sampler.Sample{ID: 1}},
				{Sample: &sampler.Sample{ID: 2, Err: fmt.Errorf("error 2")}},
				{Sample: &sampler.Sample{ID: 3, Err: fmt.Errorf("error 3")}},
				{Sample: &sampler.Sample{ID: 4}},
				{Sample: &sampler.Sample{ID: 5, Err: fmt.Errorf("error 5")}},
				{Sample: nil},
			},
			unavailable: 3,
			available:   2,
			intervals: []interval{
				{from: 1, to: 1},
				{from: 2, to: 3},
				{from: 3, to: 4},
				{from: 4, to: 5},
				{from: 5, to: 5},
			},
		},
		{
			name: "multiple samples with errors only",
			samples: []backend.SampleResult{
				{Sample: &sampler.Sample{ID: 1, Err: fmt.Errorf("error 1")}},
				{Sample: &sampler.Sample{ID: 2, Err: fmt.Errorf("error 2")}},
				{Sample: &sampler.Sample{ID: 3, Err: fmt.Errorf("error 3")}},
				{Sample: &sampler.Sample{ID: 4, Err: fmt.Errorf("error 4")}},
				{Sample: nil},
			},
			unavailable: 4,
			intervals: []interval{
				{from: 1, to: 2},
				{from: 2, to: 3},
				{from: 3, to: 4},
				{from: 4, to: 4},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler := &fakeHandler{t: t}
			tracker := &intervalTracker{handler: handler}

			for i := range test.samples {
				tracker.Collect(test.samples[i])
			}

			if handler.err != nil {
				t.Errorf("unexpected error: %v", handler.err)
			}
			if handler.openIntervalID > 0 {
				t.Errorf("unexpected open interval: %d", handler.openIntervalID)
			}
			if test.available != handler.available {
				t.Errorf("expected %d available intervals, but got: %d", test.available, handler.available)
			}
			if test.unavailable != handler.unavailable {
				t.Errorf("expected %d unavailable intervals, but got: %d", test.unavailable, handler.unavailable)
			}
			if !reflect.DeepEqual(test.intervals, handler.intervals) {
				t.Errorf("expected intervals: %v", test.intervals)
				t.Errorf("actual intervals: %v", handler.intervals)
				t.Errorf("expected the intervals to match: diff: %s", cmp.Diff(test.intervals, handler.intervals, cmp.AllowUnexported(interval{})))
			}
		})
	}
}

type interval struct {
	from, to uint64
}
type fakeHandler struct {
	t                      *testing.T
	available, unavailable int
	intervals              []interval
	openIntervalID         int
	err                    error
}

func (f *fakeHandler) Unavailable(from, to *backend.SampleResult) {
	f.unavailable++
	f.intervals = append(f.intervals, interval{from: from.Sample.ID, to: to.Sample.ID})
}
func (f *fakeHandler) Available(from, to *backend.SampleResult) {
	f.available++
	f.intervals = append(f.intervals, interval{from: from.Sample.ID, to: to.Sample.ID})
}
