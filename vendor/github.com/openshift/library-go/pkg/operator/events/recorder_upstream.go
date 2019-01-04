package events

import (
	"fmt"

	"github.com/golang/glog"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"
)

// NewKubeRecorder returns new event recorder.
func NewKubeRecorder(client corev1client.EventInterface, sourceComponentName string, involvedObjectRef *corev1.ObjectReference) Recorder {
	broadcaster := record.NewBroadcaster()
	broadcaster.StartLogging(glog.Infof)
	broadcaster.StartRecordingToSink(&corev1client.EventSinkImpl{Interface: client})
	eventRecorder := broadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: sourceComponentName})

	return &upstreamRecorder{
		eventRecorder:     eventRecorder,
		involvedObjectRef: involvedObjectRef,
	}
}

// upstreamRecorder is an implementation of Recorder interface.
type upstreamRecorder struct {
	eventRecorder     record.EventRecorder
	involvedObjectRef *corev1.ObjectReference
}

// Eventf emits the normal type event and allow formatting of message.
func (r *upstreamRecorder) Eventf(reason, messageFmt string, args ...interface{}) {
	r.Event(reason, fmt.Sprintf(messageFmt, args...))
}

// Warningf emits the warning type event and allow formatting of message.
func (r *upstreamRecorder) Warningf(reason, messageFmt string, args ...interface{}) {
	r.Warning(reason, fmt.Sprintf(messageFmt, args...))
}

// Event emits the normal type event.
func (r *upstreamRecorder) Event(reason, message string) {
	r.eventRecorder.Event(r.involvedObjectRef, corev1.EventTypeNormal, reason, message)
}

// Warning emits the warning type event.
func (r *upstreamRecorder) Warning(reason, message string) {
	r.eventRecorder.Event(r.involvedObjectRef, corev1.EventTypeWarning, reason, message)
}
