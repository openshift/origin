package monitor

import (
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
)

type IntervalCreationFunc func(intervals monitorapi.Intervals, beginning, end time.Time) monitorapi.Intervals

type SamplerFunc func(time.Time) []*monitorapi.Condition

type Interface interface {
	Intervals(from, to time.Time) monitorapi.Intervals
	Conditions(from, to time.Time) monitorapi.Intervals
}

type Recorder interface {
	Record(conditions ...monitorapi.Condition)
	RecordAt(t time.Time, conditions ...monitorapi.Condition)

	StartInterval(t time.Time, condition monitorapi.Condition) int
	EndInterval(startedInterval int, t time.Time)

	AddSampler(fn SamplerFunc)
}

type sample struct {
	at         time.Time
	conditions []*monitorapi.Condition
}
