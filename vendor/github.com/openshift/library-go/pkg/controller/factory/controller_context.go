package factory

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"github.com/openshift/library-go/pkg/operator/events"
)

// syncContext implements SyncContext and provide user access to queue and object that caused
// the sync to be triggered.
type syncContext struct {
	eventRecorder events.Recorder
	queue         workqueue.RateLimitingInterface
	queueKey      string
}

var _ SyncContext = syncContext{}

// NewSyncContext gives new sync context.
func NewSyncContext(name string, recorder events.Recorder) SyncContext {
	return syncContext{
		queue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), name),
		eventRecorder: recorder.WithComponentSuffix(strings.ToLower(name)),
	}
}

func (c syncContext) Queue() workqueue.RateLimitingInterface {
	return c.queue
}

func (c syncContext) QueueKey() string {
	return c.queueKey
}

func (c syncContext) Recorder() events.Recorder {
	return c.eventRecorder
}

func (c syncContext) isInterestingNamespace(obj interface{}, interestingNamespaces sets.String) (bool, bool) {
	ns, ok := obj.(*corev1.Namespace)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if ok {
			if ns, ok := tombstone.Obj.(*corev1.Namespace); ok {
				return true, interestingNamespaces.Has(ns.Name)
			}
		}
		return false, false
	}
	return true, interestingNamespaces.Has(ns.Name)
}

// eventHandler provides default event handler that is added to an informers passed to controller factory.
func (c syncContext) eventHandler(queueKeyFunc ObjectQueueKeyFunc, interestingNamespaces sets.String) cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			isNamespace, isInteresting := c.isInterestingNamespace(obj, interestingNamespaces)
			runtimeObj, ok := obj.(runtime.Object)
			if !ok {
				utilruntime.HandleError(fmt.Errorf("added object %+v is not runtime Object", obj))
				return
			}
			if !isNamespace || (isNamespace && isInteresting) {
				c.Queue().Add(queueKeyFunc(runtimeObj))
			}
		},
		UpdateFunc: func(old, new interface{}) {
			isNamespace, isInteresting := c.isInterestingNamespace(new, interestingNamespaces)
			runtimeObj, ok := new.(runtime.Object)
			if !ok {
				utilruntime.HandleError(fmt.Errorf("updated object %+v is not runtime Object", runtimeObj))
				return
			}
			if !isNamespace || (isNamespace && isInteresting) {
				c.Queue().Add(queueKeyFunc(runtimeObj))
			}
		},
		DeleteFunc: func(obj interface{}) {
			isNamespace, isInteresting := c.isInterestingNamespace(obj, interestingNamespaces)
			runtimeObj, ok := obj.(runtime.Object)
			if !ok {
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if ok {
					if !isNamespace || (isNamespace && isInteresting) {
						c.Queue().Add(queueKeyFunc(tombstone.Obj.(runtime.Object)))
					}
					return
				}
				utilruntime.HandleError(fmt.Errorf("updated object %+v is not runtime Object", runtimeObj))
				return
			}
			if !isNamespace || (isNamespace && isInteresting) {
				c.Queue().Add(queueKeyFunc(runtimeObj))
			}
		},
	}
}
