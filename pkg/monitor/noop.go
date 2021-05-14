package monitor

import (
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
)

type noOpMonitor struct {
}

func NewNoOpMonitor() Recorder {
	return &noOpMonitor{}
}

func (*noOpMonitor) Record(conditions ...monitorapi.Condition)                     {}
func (*noOpMonitor) RecordAt(t time.Time, conditions ...monitorapi.Condition)      {}
func (*noOpMonitor) StartInterval(t time.Time, condition monitorapi.Condition) int { return 0 }
func (*noOpMonitor) EndInterval(startedInterval int, t time.Time)                  {}
func (*noOpMonitor) AddSampler(fn SamplerFunc)                                     {}
