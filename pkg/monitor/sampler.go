package monitor

import (
	"context"
	"sync"
	"time"
)

type ConditionalSampler interface {
	ConditionWhenFailing(*Condition) SamplerFunc
}

type sampler struct {
	lock      sync.Mutex
	available bool
}

func StartSampling(ctx context.Context, recorder Recorder, interval time.Duration, sampleFn func(previous bool) (*Condition, bool)) ConditionalSampler {
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

func (s *sampler) ConditionWhenFailing(condition *Condition) SamplerFunc {
	return func(_ time.Time) []*Condition {
		if s.isAvailable() {
			return nil
		}
		return []*Condition{condition}
	}
}
