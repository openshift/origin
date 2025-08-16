package factory

import (
	"context"
	"fmt"

	"k8s.io/client-go/util/workqueue"

	"github.com/openshift/library-go/pkg/operator/events"
)

// Controller interface represents a runnable Kubernetes controller.
// Cancelling the syncContext passed will cause the controller to shutdown.
// Number of workers determine how much parallel the job processing should be.
type Controller interface {
	// Run runs the controller and blocks until the controller is finished.
	// Number of workers can be specified via workers parameter.
	// This function will return when all internal loops are finished.
	// Note that having more than one worker usually means handing parallelization of Sync().
	Run(ctx context.Context, workers int)

	// Sync contain the main controller logic.
	// This should not be called directly, but can be used in unit tests to exercise the sync.
	Sync(ctx context.Context, controllerContext SyncContext) error

	// Name returns the controller name string.
	Name() string
}

// SyncContext interface represents a context given to the Sync() function where the main controller logic happen.
// SyncContext exposes controller name and give user access to the queue (for manual requeue).
// SyncContext also provides metadata about object that informers observed as changed.
type SyncContext interface {
	// Queue gives access to controller queue. This can be used for manual requeue, although if a Sync() function return
	// an error, the object is automatically re-queued. Use with caution.
	Queue() workqueue.RateLimitingInterface

	// QueueKey represents the queue key passed to the Sync function.
	QueueKey() string

	// Recorder provide access to event recorder.
	Recorder() events.Recorder
}

// SyncFunc is a function that contain main controller logic.
// The syncContext.syncContext passed is the main controller syncContext, when cancelled it means the controller is being shut down.
// The syncContext provides access to controller name, queue and event recorder.
type SyncFunc func(ctx context.Context, controllerContext SyncContext) error

func ControllerFieldManager(controllerName, usageName string) string {
	return fmt.Sprintf("%s-%s", controllerName, usageName)
}

func ControllerInstanceName(instanceName, controllerName string) string {
	return fmt.Sprintf("%s-%s", instanceName, controllerName)
}
