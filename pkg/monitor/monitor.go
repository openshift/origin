package monitor

import (
	"context"
	"time"

	configclientset "github.com/openshift/client-go/config/clientset/versioned"
	"github.com/openshift/origin/pkg/disruption/backend"
	"github.com/openshift/origin/pkg/monitor/apiserveravailability"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitor/shutdown"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

// Monitor records events that have occurred in memory and can also periodically
// sample results.
type Monitor struct {
	adminKubeConfig                  *rest.Config
	additionalEventIntervalRecorders []StartEventIntervalRecorderFunc

	recorder Recorder
}

// NewMonitor creates a monitor with the default sampling interval.
func NewMonitor(adminKubeConfig *rest.Config, additionalEventIntervalRecorders []StartEventIntervalRecorderFunc) *Monitor {
	return &Monitor{
		adminKubeConfig:                  adminKubeConfig,
		additionalEventIntervalRecorders: additionalEventIntervalRecorders,
		recorder:                         NewRecorder(),
	}
}

var _ Interface = &Monitor{}
var _ Recorder = &Monitor{}

// Start begins monitoring the cluster referenced by the default kube configuration until context is finished.
func (m *Monitor) Start(ctx context.Context) error {
	client, err := kubernetes.NewForConfig(m.adminKubeConfig)
	if err != nil {
		return err
	}
	configClient, err := configclientset.NewForConfig(m.adminKubeConfig)
	if err != nil {
		return err
	}

	for _, additionalEventIntervalRecorder := range m.additionalEventIntervalRecorders {
		if err := additionalEventIntervalRecorder(ctx, m, m.adminKubeConfig, backend.ExternalLoadBalancerType); err != nil {
			return err
		}
	}

	// read the state of the cluster apiserver client access issues *before* any test (like upgrade) begins
	intervals, err := apiserveravailability.APIServerAvailabilityIntervalsFromCluster(client, time.Time{}, time.Time{})
	if err != nil {
		klog.Errorf("error reading initial apiserver availability: %v", err)
	}
	m.AddIntervals(intervals...)

	startPodMonitoring(ctx, m, client)
	startNodeMonitoring(ctx, m, client)
	startEventMonitoring(ctx, m, client)
	shutdown.StartMonitoringGracefulShutdownEvents(ctx, m, client)

	// add interval creation at the same point where we add the monitors
	startClusterOperatorMonitoring(ctx, m, configClient)
	return nil
}

func (m *Monitor) CurrentResourceState() monitorapi.ResourcesMap {
	return m.recorder.CurrentResourceState()
}

func (m *Monitor) RecordResource(resourceType string, obj runtime.Object) {
	m.recorder.RecordResource(resourceType, obj)
}

// Record captures one or more conditions at the current time. All conditions are recorded
// in monotonic order as EventInterval objects.
func (m *Monitor) Record(conditions ...monitorapi.Condition) {
	m.recorder.Record(conditions...)
}

// AddIntervals provides a mechanism to directly inject eventIntervals
func (m *Monitor) AddIntervals(eventIntervals ...monitorapi.EventInterval) {
	m.recorder.AddIntervals(eventIntervals...)
}

// StartInterval inserts a record at time t with the provided condition and returns an opaque
// locator to the interval. The caller may close the sample at any point by invoking EndInterval().
func (m *Monitor) StartInterval(t time.Time, condition monitorapi.Condition) int {
	return m.recorder.StartInterval(t, condition)
}

// EndInterval updates the To of the interval started by StartInterval if it is greater than
// the from.
func (m *Monitor) EndInterval(startedInterval int, t time.Time) {
	m.recorder.EndInterval(startedInterval, t)
}

// RecordAt captures one or more conditions at the provided time. All conditions are recorded
// as EventInterval objects.
func (m *Monitor) RecordAt(t time.Time, conditions ...monitorapi.Condition) {
	m.recorder.RecordAt(t, conditions...)
}

// Intervals returns all events that occur between from and to, including
// any sampled conditions that were encountered during that period.
// Intervals are returned in order of their occurrence. The returned slice
// is a copy of the monitor's state and is safe to update.
func (m *Monitor) Intervals(from, to time.Time) monitorapi.Intervals {
	return m.recorder.Intervals(from, to)
}
