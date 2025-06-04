package events

import (
	"context"
	"fmt"
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

type inMemoryEventRecorder struct {
	events []*corev1.Event
	source string
	ctx    context.Context
	sync.Mutex
}

// inMemoryDummyObjectReference is used for fake events.
var inMemoryDummyObjectReference = corev1.ObjectReference{
	Kind:       "Pod",
	Namespace:  "dummy",
	Name:       "dummy",
	APIVersion: "v1",
}

type InMemoryRecorder interface {
	Events() []*corev1.Event
	Recorder
}

// NewInMemoryRecorder provides event recorder that stores all events recorded in memory and allow to replay them using the Events() method.
// This recorder should be only used in unit tests.
func NewInMemoryRecorder(sourceComponent string) InMemoryRecorder {
	return &inMemoryEventRecorder{events: []*corev1.Event{}, source: sourceComponent}
}

func (r *inMemoryEventRecorder) ComponentName() string {
	return r.source
}

func (r *inMemoryEventRecorder) Shutdown() {}

func (r *inMemoryEventRecorder) ForComponent(component string) Recorder {
	r.Lock()
	defer r.Unlock()
	r.source = component
	return r
}

func (r *inMemoryEventRecorder) WithContext(ctx context.Context) Recorder {
	r.ctx = ctx
	return r
}

func (r *inMemoryEventRecorder) WithComponentSuffix(suffix string) Recorder {
	return r.ForComponent(fmt.Sprintf("%s-%s", r.ComponentName(), suffix))
}

// Events returns list of recorded events
func (r *inMemoryEventRecorder) Events() []*corev1.Event {
	return r.events
}

func (r *inMemoryEventRecorder) Event(reason, message string) {
	r.Lock()
	defer r.Unlock()
	event := makeEvent(&inMemoryDummyObjectReference, r.source, corev1.EventTypeNormal, reason, message)
	r.events = append(r.events, event)
}

func (r *inMemoryEventRecorder) Eventf(reason, messageFmt string, args ...interface{}) {
	r.Event(reason, fmt.Sprintf(messageFmt, args...))
}

func (r *inMemoryEventRecorder) Warning(reason, message string) {
	r.Lock()
	defer r.Unlock()
	event := makeEvent(&inMemoryDummyObjectReference, r.source, corev1.EventTypeWarning, reason, message)
	klog.Info(event.String())
	r.events = append(r.events, event)
}

func (r *inMemoryEventRecorder) Warningf(reason, messageFmt string, args ...interface{}) {
	r.Warning(reason, fmt.Sprintf(messageFmt, args...))
}
