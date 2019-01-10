package eventstesting

import (
	"fmt"
	"testing"

	"github.com/openshift/library-go/pkg/operator/events"
)

type TestingEventRecorder struct {
	t *testing.T
}

// NewTestingEventRecorder provides event recorder that will log all recorded events to the error log.
func NewTestingEventRecorder(t *testing.T) events.Recorder {
	return &TestingEventRecorder{t: t}
}

func (r *TestingEventRecorder) Event(reason, message string) {
	r.t.Logf("Event: %v: %v", reason, message)
}

func (r *TestingEventRecorder) Eventf(reason, messageFmt string, args ...interface{}) {
	r.Event(reason, fmt.Sprintf(messageFmt, args...))
}

func (r *TestingEventRecorder) Warning(reason, message string) {
	r.t.Logf("Warning: %v: %v", reason, message)
}

func (r *TestingEventRecorder) Warningf(reason, messageFmt string, args ...interface{}) {
	r.Warning(reason, fmt.Sprintf(messageFmt, args...))
}
