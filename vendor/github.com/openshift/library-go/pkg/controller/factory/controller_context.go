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

// eventHandler provides default event handler that is added to an informers passed to controller factory.
func (c syncContext) eventHandler(queueKeysFunc ObjectQueueKeysFunc, filter EventFilterFunc) cache.ResourceEventHandler {
	resourceEventHandler := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			runtimeObj, ok := obj.(runtime.Object)
			if !ok {
				utilruntime.HandleError(fmt.Errorf("added object %+v is not runtime Object", obj))
				return
			}
			c.enqueueKeys(queueKeysFunc(runtimeObj)...)
		},
		UpdateFunc: func(old, new interface{}) {
			runtimeObj, ok := new.(runtime.Object)
			if !ok {
				utilruntime.HandleError(fmt.Errorf("updated object %+v is not runtime Object", runtimeObj))
				return
			}
			c.enqueueKeys(queueKeysFunc(runtimeObj)...)
		},
		DeleteFunc: func(obj interface{}) {
			runtimeObj, ok := obj.(runtime.Object)
			if !ok {
				if tombstone, ok := obj.(cache.DeletedFinalStateUnknown); ok {
					c.enqueueKeys(queueKeysFunc(tombstone.Obj.(runtime.Object))...)

					return
				}
				utilruntime.HandleError(fmt.Errorf("updated object %+v is not runtime Object", runtimeObj))
				return
			}
			c.enqueueKeys(queueKeysFunc(runtimeObj)...)
		},
	}
	if filter == nil {
		return resourceEventHandler
	}
	return cache.FilteringResourceEventHandler{
		FilterFunc: filter,
		Handler:    resourceEventHandler,
	}
}

func (c syncContext) enqueueKeys(keys ...string) {
	for _, qKey := range keys {
		c.queue.Add(qKey)
	}
}

// namespaceChecker returns a function which returns true if an inpuut obj
// (or its tombstone) is a namespace  and it matches a name of any namespaces
// that we are interested in
func namespaceChecker(interestingNamespaces []string) func(obj interface{}) bool {
	interestingNamespacesSet := sets.NewString(interestingNamespaces...)

	return func(obj interface{}) bool {
		ns, ok := obj.(*corev1.Namespace)
		if ok {
			return interestingNamespacesSet.Has(ns.Name)
		}

		// the object might be getting deleted
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if ok {
			if ns, ok := tombstone.Obj.(*corev1.Namespace); ok {
				return interestingNamespacesSet.Has(ns.Name)
			}
		}
		return false
	}
}
