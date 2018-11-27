package events

import (
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
)

// Recorder is a simple event recording interface.
type Recorder interface {
	Event(reason, message string) error
	Eventf(reason, messageFmt string, args ...interface{}) error
	Warning(reason, message string) error
	Warningf(reason, messageFmt string, args ...interface{}) error
}

// NewRecorder returns new event recorder.
func NewRecorder(client corev1client.EventInterface, sourceComponentName string, involvedObjectRef *corev1.ObjectReference) Recorder {
	return &recorder{
		eventClient:       client,
		involvedObjectRef: involvedObjectRef,
		sourceComponent:   sourceComponentName,
	}
}

// recorder is an implementation of Recorder interface.
type recorder struct {
	eventClient       corev1client.EventInterface
	involvedObjectRef *corev1.ObjectReference
	sourceComponent   string
}

// Event emits the normal type event and allow formatting of message.
func (r *recorder) Eventf(reason, messageFmt string, args ...interface{}) error {
	return r.Event(reason, fmt.Sprintf(messageFmt, args...))
}

// Warning emits the warning type event and allow formatting of message.
func (r *recorder) Warningf(reason, messageFmt string, args ...interface{}) error {
	return r.Warning(reason, fmt.Sprintf(messageFmt, args...))
}

// Event emits the normal type event.
func (r *recorder) Event(reason, message string) error {
	_, err := r.eventClient.Create(r.makeEvent(r.involvedObjectRef, corev1.EventTypeNormal, reason, message))
	return err
}

// Warning emits the warning type event.
func (r *recorder) Warning(reason, message string) error {
	_, err := r.eventClient.Create(r.makeEvent(r.involvedObjectRef, corev1.EventTypeWarning, reason, message))
	return err
}

func (r recorder) makeEvent(involvedObjRef *corev1.ObjectReference, eventType, reason, message string) *corev1.Event {
	currentTime := metav1.Time{Time: time.Now()}
	event := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%v.%x", involvedObjRef.Name, currentTime.UnixNano()),
			Namespace: involvedObjRef.Namespace,
		},
		InvolvedObject: *involvedObjRef,
		Reason:         reason,
		Message:        message,
		Type:           eventType,
		Count:          1,
		FirstTimestamp: currentTime,
		LastTimestamp:  currentTime,
		EventTime:      metav1.MicroTime{Time: currentTime.Time},
	}
	if len(r.sourceComponent) > 0 {
		event.Source.Component = r.sourceComponent
	}
	return event
}
