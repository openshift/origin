package monitor

import (
	"context"
	"sync"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
)

type sampler struct {
	onFailing *monitorapi.Condition
	interval  time.Duration
	recorder  Recorder
	sampleFn  func(previous bool) (*monitorapi.Condition, bool)

	lock      sync.Mutex
	available bool
}

func NewSampler(recorder Recorder, interval time.Duration, sampleFn SampleFunc) ConditionalSampler {
	s := &sampler{
		available: true,
		recorder:  recorder,
		interval:  interval,
		sampleFn:  sampleFn,
	}
	return s
}

func (s *sampler) run(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	var lastInterval int = -1
	for {
		previousSampleWasAvailable := s.isAvailable()
		// the sampleFn may take a significant period of time to run.  In such a case, we want our start interval
		// for when a failure started to be the time when the request was first made, not the time when the call
		// returned.  Imagine a timeout set on a DNS lookup of 30s: when the GET finally fails and returns, the outage
		// was actually 30s before.
		startTime := time.Now().UTC()
		condition, currentSampleIsAvailable := s.sampleFn(previousSampleWasAvailable)
		if condition != nil {
			s.recorder.RecordAt(startTime, *condition)
		}
		if s.onFailing != nil {
			switch {
			case !previousSampleWasAvailable && currentSampleIsAvailable:
				if lastInterval != -1 {
					s.recorder.EndInterval(lastInterval, time.Now().UTC())
				}
			case previousSampleWasAvailable && !currentSampleIsAvailable:
				if condition != nil { // if an edge condition is provided, use this to load the interval
					lastInterval = s.recorder.StartInterval(startTime, *condition)
				} else {
					lastInterval = s.recorder.StartInterval(startTime, *s.onFailing)
				}
			}
		}
		s.setAvailable(currentSampleIsAvailable)

		select {
		case <-ticker.C:
		case <-ctx.Done():
			return
		}
	}
}

func (s *sampler) isAvailable() bool {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.available
}
func (s *sampler) setAvailable(b bool) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.available = b
}

func (s *sampler) ConditionWhenFailing(ctx context.Context, condition *monitorapi.Condition) SamplerFunc {
	go s.run(ctx)
	return func(_ time.Time) []*monitorapi.Condition {
		if s.isAvailable() {
			return nil
		}
		return []*monitorapi.Condition{condition}
	}
}

func (s *sampler) WhenFailing(ctx context.Context, condition *monitorapi.Condition) {
	s.onFailing = condition
	s.run(ctx)
}
