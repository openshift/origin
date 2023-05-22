package disruption

import (
	"fmt"

	"github.com/openshift/origin/pkg/disruption/backend"
	"github.com/openshift/origin/pkg/monitor/backenddisruption"
	"github.com/openshift/origin/pkg/monitor/monitorapi"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/events"
	"k8s.io/kubernetes/test/e2e/framework"
)

// newCIHandler returns a new intervalHandler instance
// that can record the availability and unavailability
// interval in CI using the Monitor API and the event handler.
//
//	monitor: Monitor API to start and end an interval in CI
//	eventRecorder: to create events associated with the intervals
//	locator: the CI locator assigned to this disruption test
//	name: name of the disruption test
//	connType: user specified BackendConnectionType used in this test
func newCIHandler(monitor backend.Monitor, eventRecorder events.EventRecorder, locator, name string,
	connType monitorapi.BackendConnectionType) *ciHandler {
	return &ciHandler{
		monitor:        monitor,
		eventRecorder:  eventRecorder,
		locator:        locator,
		name:           name,
		connType:       connType,
		openIntervalID: -1,
	}
}

var _ intervalHandler = &ciHandler{}
var _ backend.WantEventRecorderAndMonitor = &ciHandler{}

// ciHandler records the availability and unavailability interval in CI
type ciHandler struct {
	monitor       backend.Monitor
	eventRecorder events.EventRecorder
	locator, name string
	connType      monitorapi.BackendConnectionType

	openIntervalID int
}

// SetEventRecorder sets the event recorder
func (m *ciHandler) SetEventRecorder(recorder events.EventRecorder) {
	m.eventRecorder = recorder
}

// SetMonitor sets the interval recorder provided by the monitor API
func (m *ciHandler) SetMonitor(monitor backend.Monitor) {
	m.monitor = monitor
}

// UnavailableStarted records an unavailable disruption interval in CI
func (m *ciHandler) UnavailableStarted(result backend.SampleResult) {
	s := result.Sample
	fields := fmt.Sprintf("sample-id=%d %s", s.ID, result.String())
	message, eventReason, level := backenddisruption.DisruptionBegan(m.locator, m.connType, fmt.Errorf("%w - %s", result.AggregateErr(), fields))

	framework.Logf(message)
	m.eventRecorder.Eventf(
		&v1.ObjectReference{Kind: "OpenShiftTest", Namespace: "kube-system", Name: m.name},
		nil, v1.EventTypeWarning, eventReason, "detected", message)

	condition := monitorapi.Condition{
		Level:   level,
		Locator: m.locator,
		Message: message,
	}
	m.openIntervalID = m.monitor.StartInterval(s.StartedAt, condition)
}

// AvailableStarted records an available again interval in CI
func (m *ciHandler) AvailableStarted(result backend.SampleResult) {
	message := backenddisruption.DisruptionEndedMessage(m.locator, m.connType)
	framework.Logf(message)

	m.eventRecorder.Eventf(
		&v1.ObjectReference{Kind: "OpenShiftTest", Namespace: "kube-system", Name: m.name}, nil,
		v1.EventTypeNormal, backenddisruption.DisruptionEndedEventReason, "detected", message)
	condition := monitorapi.Condition{
		Level:   monitorapi.Info,
		Locator: m.locator,
		Message: message,
	}
	m.openIntervalID = m.monitor.StartInterval(result.Sample.StartedAt, condition)
}

// CloseInterval closes an open interval, if any.
func (m *ciHandler) CloseInterval(result backend.SampleResult) {
	if m.openIntervalID >= 0 {
		m.monitor.EndInterval(m.openIntervalID, result.Sample.StartedAt)
	}
	m.openIntervalID = -1
}
