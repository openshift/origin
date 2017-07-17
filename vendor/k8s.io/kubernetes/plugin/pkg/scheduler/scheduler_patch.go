package scheduler

import (
	"k8s.io/kubernetes/plugin/pkg/scheduler/metrics"
)

func NewFromConfig(cfg *Config) *Scheduler {
	// From this point on the config is immutable to the outside.
	s := &Scheduler{
		config: cfg,
	}
	metrics.Register()
	return s
}
