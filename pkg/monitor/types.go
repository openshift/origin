package monitor

import (
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
)

type SamplerFunc func(time.Time) []*monitorapi.Condition

type Interface interface {
	EventIntervals(from, to time.Time) monitorapi.EventIntervals
	Conditions(from, to time.Time) monitorapi.EventIntervals
}

type Recorder interface {
	Record(conditions ...monitorapi.Condition)
	AddSampler(fn SamplerFunc)
}

type sample struct {
	at         time.Time
	conditions []*monitorapi.Condition
}
