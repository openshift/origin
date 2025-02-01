package events

import (
	"context"
	"fmt"
	"k8s.io/utils/clock"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

type LoggingEventRecorder struct {
	component string
	clock     clock.PassiveClock
	ctx       context.Context
}

func (r *LoggingEventRecorder) WithContext(ctx context.Context) Recorder {
	r.ctx = ctx
	return r
}

// NewLoggingEventRecorder provides event recorder that will log all recorded events via klog.
func NewLoggingEventRecorder(component string, clock clock.PassiveClock) Recorder {
	return &LoggingEventRecorder{
		component: component,
		clock:     clock,
	}
}

func (r *LoggingEventRecorder) ComponentName() string {
	return r.component
}

func (r *LoggingEventRecorder) ForComponent(component string) Recorder {
	newRecorder := *r
	newRecorder.component = component
	return &newRecorder
}

func (r *LoggingEventRecorder) Shutdown() {}

func (r *LoggingEventRecorder) WithComponentSuffix(suffix string) Recorder {
	return r.ForComponent(fmt.Sprintf("%s-%s", r.ComponentName(), suffix))
}

func (r *LoggingEventRecorder) Event(reason, message string) {
	event := makeEvent(r.clock, &inMemoryDummyObjectReference, "", corev1.EventTypeNormal, reason, message)
	klog.Info(event.String())
}

func (r *LoggingEventRecorder) Eventf(reason, messageFmt string, args ...interface{}) {
	r.Event(reason, fmt.Sprintf(messageFmt, args...))
}

func (r *LoggingEventRecorder) Warning(reason, message string) {
	event := makeEvent(r.clock, &inMemoryDummyObjectReference, "", corev1.EventTypeWarning, reason, message)
	klog.Warning(event.String())
}

func (r *LoggingEventRecorder) Warningf(reason, messageFmt string, args ...interface{}) {
	r.Warning(reason, fmt.Sprintf(messageFmt, args...))
}
