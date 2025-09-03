package disruption

import (
	"fmt"
	"time"

	"k8s.io/klog/v2"

	"github.com/openshift/origin/pkg/disruption/backend"
	"github.com/openshift/origin/pkg/monitor/backenddisruption"
	"github.com/openshift/origin/pkg/monitor/monitorapi"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/events"
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
func newCIHandler(descriptor backend.TestDescriptor, monitor monitorapi.RecorderWriter, eventRecorder events.EventRecorder) *ciHandler {
	return &ciHandler{
		descriptor:      descriptor,
		monitorRecorder: monitor,
		eventRecorder:   eventRecorder,
		openIntervalID:  -1,
	}
}

var _ intervalHandler = &ciHandler{}
var _ backend.WantEventRecorderAndMonitorRecorder = &ciHandler{}

// ciHandler records the availability and unavailability interval in CI
type ciHandler struct {
	descriptor      backend.TestDescriptor
	monitorRecorder monitorapi.RecorderWriter
	eventRecorder   events.EventRecorder

	openIntervalID int
	last           backend.SampleResult
}

// SetEventRecorder sets the event recorder
func (h *ciHandler) SetEventRecorder(recorder events.EventRecorder) {
	h.eventRecorder = recorder
}

// SetMonitor sets the interval recorder provided by the monitor API
func (h *ciHandler) SetMonitorRecorder(monitorRecorder monitorapi.RecorderWriter) {
	h.monitorRecorder = monitorRecorder
}

// Unavailable is called for a disruption interval when we see
// a series of failed samples in this range [from ... to).
//
//	a) either from or to must not be nil
//	b) for a window with a single sample, from and to can refer
//	   to the same sample in question.
func (h *ciHandler) Unavailable(from, to *backend.SampleResult) {
	fs, ts := from.Sample, to.Sample
	info := fmt.Sprintf("sample-id=%d %s", fs.ID, from.String())
	if ts.ID-fs.ID > 1 {
		// we have multiple failed samples with the same error
		info = fmt.Sprintf("range=[%d-%d] %s", fs.ID, ts.ID, info)
	}
	message, eventReason, level := backenddisruption.DisruptionBegan(h.descriptor.DisruptionLocator().OldLocator(),
		h.descriptor.GetConnectionType(), fmt.Errorf("%w - %s", from.AggregateErr(), info), "no-audit-id")

	klog.V(4).Info(message)
	h.eventRecorder.Eventf(
		&v1.ObjectReference{Kind: "OpenShiftTest", Namespace: "kube-system", Name: h.descriptor.Name()},
		nil, v1.EventTypeWarning, string(eventReason), "detected", message.BuildString())

	interval := monitorapi.NewInterval(h.getSource(), level).Locator(h.descriptor.DisruptionLocator()).
		Display().
		Message(message).Build(fs.StartedAt, time.Time{})
	openIntervalID := h.monitorRecorder.StartInterval(interval)
	// TODO: unlikely in the real world, if from == to for some reason,
	//  then we are recording a zero second unavailable window.
	h.monitorRecorder.EndInterval(openIntervalID, ts.StartedAt)
}

// Available is called when a disruption interval ends and we see
// a series of successful samples in this range [from ... to).
//
//	a) either from or to must not be nil
//	b) for a window with a single sample, from and to can refer
//	   to the same sample in question.
func (h *ciHandler) Available(from, to *backend.SampleResult) {
	fs, ts := from.Sample, to.Sample
	message := backenddisruption.DisruptionEndedMessage(h.descriptor.DisruptionLocator().OldLocator(),
		h.descriptor.GetConnectionType())
	klog.V(4).Info(message)

	h.eventRecorder.Eventf(
		&v1.ObjectReference{Kind: "OpenShiftTest", Namespace: "kube-system", Name: h.descriptor.Name()}, nil,
		v1.EventTypeNormal, string(monitorapi.DisruptionEndedEventReason), "detected", message.BuildString())
	interval := monitorapi.NewInterval(h.getSource(), monitorapi.Info).Locator(h.descriptor.DisruptionLocator()).
		Message(message).Build(fs.StartedAt, time.Time{})
	openIntervalID := h.monitorRecorder.StartInterval(interval)
	h.monitorRecorder.EndInterval(openIntervalID, ts.StartedAt)
}

// getSource returns the source of the interval based on the load balancer type. Localhost disruptions need to be separated, as in some cases these are expected
func (h *ciHandler) getSource() monitorapi.IntervalSource {
	if h.descriptor.GetLoadBalancerType() == backend.LocalhostType {
		return monitorapi.SourceDisruptionLocalhost
	}
	return monitorapi.SourceDisruption
}
