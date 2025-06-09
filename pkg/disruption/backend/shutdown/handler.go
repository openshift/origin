package shutdown

import (
	"fmt"
	"sync"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/events"
	"k8s.io/kubernetes/test/e2e/framework"

	"github.com/openshift/origin/pkg/disruption/backend"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
)

func newCIShutdownIntervalHandler(descriptor backend.TestDescriptor, monitorRecorder monitorapi.RecorderWriter, eventRecorder events.EventRecorder) *ciShutdownIntervalHandler {
	return &ciShutdownIntervalHandler{
		monitorRecorder: monitorRecorder,
		eventRecorder:   eventRecorder,
		descriptor:      descriptor,
	}
}

var _ shutdownIntervalHandler = &ciShutdownIntervalHandler{}
var _ backend.WantEventRecorderAndMonitorRecorder = &ciShutdownIntervalHandler{}

type ciShutdownIntervalHandler struct {
	descriptor backend.TestDescriptor

	lock            sync.Mutex
	monitorRecorder monitorapi.RecorderWriter
	eventRecorder   events.EventRecorder
}

// SetEventRecorder sets the event recorder
func (h *ciShutdownIntervalHandler) SetEventRecorder(recorder events.EventRecorder) {
	h.lock.Lock()
	defer h.lock.Unlock()
	h.eventRecorder = recorder
}

// SetMonitor sets the interval recorder provided by the monitor API
func (h *ciShutdownIntervalHandler) SetMonitorRecorder(monitorRecorder monitorapi.RecorderWriter) {
	h.lock.Lock()
	defer h.lock.Unlock()
	h.monitorRecorder = monitorRecorder
}

func (h *ciShutdownIntervalHandler) Handle(shutdown *shutdownInterval) {
	const (
		reason = "GracefulShutdownInterval"
	)

	level := monitorapi.Info
	message := "graceful shutdown interval"
	if len(shutdown.Failures) > 0 {
		level = monitorapi.Error
		message = fmt.Sprintf("%s: %d failures", message, len(shutdown.Failures))
	}
	message = fmt.Sprintf("%s: load balancer took new(%s) reused(%s) to switch to a new host", message,
		shutdown.MaxElapsedWithNewConnection.Round(time.Second), shutdown.MaxElapsedWithConnectionReuse.Round(time.Second))
	message = fmt.Sprintf("reason/%s locator/%s %s: %s", reason, h.descriptor.ShutdownLocator(), message, shutdown.String())
	framework.Logf("%s", message)

	if level == monitorapi.Error {
		h.eventRecorder.Eventf(
			&v1.ObjectReference{Kind: "OpenShiftTest", Namespace: "kube-system", Name: h.descriptor.Name()},
			nil, v1.EventTypeWarning, reason, "detected", message)
	}
	interval := monitorapi.NewInterval(monitorapi.SourceAPIServerShutdown, level).
		Locator(h.descriptor.ShutdownLocator()).Display().
		Message(monitorapi.NewMessage().HumanMessage(message).Reason(reason)).Build(shutdown.From, shutdown.To)
	intervalID := h.monitorRecorder.StartInterval(interval)
	h.monitorRecorder.EndInterval(intervalID, shutdown.To)
}
