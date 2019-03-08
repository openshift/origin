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
	return (&upstreamRecorder{
		client:            client,
		component:         sourceComponentName,
		involvedObjectRef: involvedObjectRef,
	}).ForComponent(sourceComponentName)
}

// upstreamRecorder is an implementation of Recorder interface.
type upstreamRecorder struct {
	client            corev1client.EventInterface
	component         string
	broadcaster       record.EventBroadcaster
	eventRecorder     record.EventRecorder
	involvedObjectRef *corev1.ObjectReference
}

func (r *upstreamRecorder) ForComponent(componentName string) Recorder {
	newRecorderForComponent := *r
	broadcaster := record.NewBroadcaster()
	broadcaster.StartLogging(glog.Infof)
	broadcaster.StartRecordingToSink(&corev1client.EventSinkImpl{Interface: newRecorderForComponent.client})

	newRecorderForComponent.eventRecorder = broadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: componentName})
	newRecorderForComponent.component = componentName

	return &newRecorderForComponent
}

func (r *upstreamRecorder) WithComponentSuffix(suffix string) Recorder {
	return r.ForComponent(fmt.Sprintf("%s-%s", r.ComponentName(), suffix))
}

func (r *upstreamRecorder) ComponentName() string {
	return r.component
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
