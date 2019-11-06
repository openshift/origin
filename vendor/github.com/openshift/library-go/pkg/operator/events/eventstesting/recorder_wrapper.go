package eventstesting

import (
	"testing"

	"github.com/openshift/library-go/pkg/operator/events"
)

type EventRecorder struct {
	realEventRecorder    events.Recorder
	testingEventRecorder *TestingEventRecorder
}

func NewEventRecorder(t *testing.T, r events.Recorder) events.Recorder {
	return &EventRecorder{
		testingEventRecorder: NewTestingEventRecorder(t).(*TestingEventRecorder),
		realEventRecorder:    r,
	}
}

func (e *EventRecorder) Event(reason, message string) {
	e.realEventRecorder.Event(reason, message)
	e.testingEventRecorder.Event(reason, message)
}

func (e *EventRecorder) Eventf(reason, messageFmt string, args ...interface{}) {
	e.realEventRecorder.Eventf(reason, messageFmt, args...)
	e.testingEventRecorder.Eventf(reason, messageFmt, args...)
}

func (e *EventRecorder) Warning(reason, message string) {
	e.realEventRecorder.Warning(reason, message)
	e.testingEventRecorder.Warning(reason, message)
}

func (e *EventRecorder) Warningf(reason, messageFmt string, args ...interface{}) {
	e.realEventRecorder.Warningf(reason, messageFmt, args...)
	e.testingEventRecorder.Warningf(reason, messageFmt, args...)
}

func (e *EventRecorder) ForComponent(componentName string) events.Recorder {
	return e
}

func (e *EventRecorder) WithComponentSuffix(componentNameSuffix string) events.Recorder {
	return e
}

func (e *EventRecorder) ComponentName() string {
	return "test-recorder"
}
