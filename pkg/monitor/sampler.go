package monitor

import (
	"context"
	"sync"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
)

type ConditionalSampler interface {
	ConditionWhenFailing(*monitorapi.Condition) SamplerFunc
}

type sampler struct {
	lock      sync.Mutex
	available bool
}

func StartSampling(ctx context.Context, recorder Recorder, interval time.Duration, sampleFn func(previous bool) (*monitorapi.Condition, bool)) ConditionalSampler {
	s := &sampler{
		available: true,
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			success := s.isAvailable()
			condition, ok := sampleFn(success)
			if condition != nil {
				recorder.Record(*condition)
			}
			s.setAvailable(ok)

			select {
			case <-ticker.C:
			case <-ctx.Done():
				return
			}
		}
	}()

	return s
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

func (s *sampler) ConditionWhenFailing(condition *monitorapi.Condition) SamplerFunc {
	return func(_ time.Time) []*monitorapi.Condition {
		if s.isAvailable() {
			return nil
		}
		return []*monitorapi.Condition{condition}
	}
}
