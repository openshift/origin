package calls

import (
	"github.com/mesos/mesos-go/api/v1/lib/scheduler"
)

// AckError wraps a caller-generated error and tracks the call that failed.
// It may be reported for either a task status ACK error, or an offer operation
// status ACK error.
type AckError struct {
	Ack   *scheduler.Call // Ack is REQUIRED
	Cause error           // Cause is REQUIRED
}

func (err *AckError) Error() string { return err.Cause.Error() }
