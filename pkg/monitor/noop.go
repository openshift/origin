package monitor

import (
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type noOpMonitor struct {
}

func NewNoOpMonitor() Recorder {
	return &noOpMonitor{}
}

func (*noOpMonitor) RecordResource(resourceType string, obj runtime.Object)                      {}
func (*noOpMonitor) Record(conditions ...monitorapi.Condition)                                   {}
func (*noOpMonitor) RecordAt(t time.Time, obj *corev1.Event, conditions ...monitorapi.Condition) {}
func (*noOpMonitor) StartInterval(t time.Time, condition monitorapi.Condition) int               { return 0 }
func (*noOpMonitor) EndInterval(startedInterval int, t time.Time)                                {}
func (*noOpMonitor) AddSampler(fn SamplerFunc)                                                   {}
