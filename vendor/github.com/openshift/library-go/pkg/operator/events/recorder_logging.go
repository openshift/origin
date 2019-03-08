package events

import (
	"fmt"

	"github.com/golang/glog"
	corev1 "k8s.io/api/core/v1"
)

type LoggingEventRecorder struct {
	component string
}

// NewLoggingEventRecorder provides event recorder that will log all recorded events via glog.
func NewLoggingEventRecorder(component string) Recorder {
	return &LoggingEventRecorder{component: component}
}

func (r *LoggingEventRecorder) ComponentName() string {
	return r.component
}

func (r *LoggingEventRecorder) ForComponent(component string) Recorder {
	newRecorder := *r
	newRecorder.component = component
	return &newRecorder
}

func (r *LoggingEventRecorder) WithComponentSuffix(suffix string) Recorder {
	return r.ForComponent(fmt.Sprintf("%s-%s", r.ComponentName(), suffix))
}

func (r *LoggingEventRecorder) Event(reason, message string) {
	event := makeEvent(&inMemoryDummyObjectReference, "", corev1.EventTypeNormal, reason, message)
	glog.Info(event.String())
}

func (r *LoggingEventRecorder) Eventf(reason, messageFmt string, args ...interface{}) {
	r.Event(reason, fmt.Sprintf(messageFmt, args...))
}

func (r *LoggingEventRecorder) Warning(reason, message string) {
	event := makeEvent(&inMemoryDummyObjectReference, "", corev1.EventTypeWarning, reason, message)
	glog.Warning(event.String())
}

func (r *LoggingEventRecorder) Warningf(reason, messageFmt string, args ...interface{}) {
	r.Warning(reason, fmt.Sprintf(messageFmt, args...))
}
