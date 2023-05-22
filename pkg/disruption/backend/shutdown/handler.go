package shutdown

import (
	"fmt"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/events"
	"k8s.io/kubernetes/test/e2e/framework"

	"github.com/openshift/origin/pkg/disruption/backend"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
)

func newCIShutdownIntervalHandler(monitor backend.Monitor, eventRecorder events.EventRecorder, locator, name string) *ciShutdownIntervalHandler {
	return &ciShutdownIntervalHandler{
		monitor:       monitor,
		eventRecorder: eventRecorder,
		locator:       locator,
		name:          name,
	}
}

var _ shutdownIntervalHandler = &ciShutdownIntervalHandler{}
var _ backend.WantEventRecorderAndMonitor = &ciShutdownIntervalHandler{}

type ciShutdownIntervalHandler struct {
	monitor       backend.Monitor
	eventRecorder events.EventRecorder
	locator, name string
	connType      monitorapi.BackendConnectionType
}

// SetEventRecorder sets the event recorder
func (m *ciShutdownIntervalHandler) SetEventRecorder(recorder events.EventRecorder) {
	m.eventRecorder = recorder
}

// SetMonitor sets the interval recorder provided by the monitor API
func (m *ciShutdownIntervalHandler) SetMonitor(monitor backend.Monitor) {
	m.monitor = monitor
}

func (m *ciShutdownIntervalHandler) Handle(shutdown *shutdownInterval) {
	const (
		reason = "GracefulShutdownDisruption"
	)

	level := monitorapi.Info
	message := "graceful shutdown window has no error"
	if len(shutdown.Failures) > 0 {
		level = monitorapi.Error
		message = "graceful shutdown window has error(s)"
	}
	message = fmt.Sprintf("reason/%s locator/%s %s: %s", reason, m.locator, message, shutdown.String())
	framework.Logf(message)

	if level == monitorapi.Error {
		m.eventRecorder.Eventf(
			&v1.ObjectReference{Kind: "OpenShiftTest", Namespace: "kube-system", Name: m.name},
			nil, v1.EventTypeWarning, reason, "detected", message)
	}
	condition := monitorapi.Condition{
		Level:   level,
		Locator: m.locator,
		Message: message,
	}
	intervalID := m.monitor.StartInterval(shutdown.FirstSampleSeenAt, condition)
	m.monitor.EndInterval(intervalID, shutdown.LastSampleSeenAt)
}
