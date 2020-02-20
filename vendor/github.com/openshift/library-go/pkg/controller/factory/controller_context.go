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
	queue         workqueue.RateLimitingInterface
	eventRecorder events.Recorder

	// queueRuntimeObject holds the object we got from informer
	// There is no direct access to this object to prevent cache mutation.
	queueRuntimeObject runtime.Object
}

var _ SyncContext = syncContext{}

// NewSyncContext gives new sync context.
func NewSyncContext(name string, recorder events.Recorder) SyncContext {
	return syncContext{
		queue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), name),
		eventRecorder: recorder.WithComponentSuffix(strings.ToLower(name)),
	}
}

// GetObject gives the object from the queue.
// For controllers generated without WithRuntimeObject() this always return nil.
func (c syncContext) GetObject() runtime.Object {
	if c.queueRuntimeObject == nil {
		return nil
	}
	return c.queueRuntimeObject.DeepCopyObject()
}

func (c syncContext) Queue() workqueue.RateLimitingInterface {
	return c.queue
}

func (c syncContext) Recorder() events.Recorder {
	return c.eventRecorder
}

// withRuntimeObject make a copy of existing sync context and set the queueRuntimeObject.
func (c syncContext) withRuntimeObject(obj runtime.Object) SyncContext {
	return syncContext{
		eventRecorder:      c.Recorder(),
		queue:              c.Queue(),
		queueRuntimeObject: obj,
	}
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
func (c syncContext) eventHandler(keyName string, objectQueue bool, interestingNamespaces sets.String) cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			isNamespace, isInteresting := c.isInterestingNamespace(obj, interestingNamespaces)
			if !objectQueue {
				if !isNamespace {
					c.Queue().Add(keyName)
				} else if isInteresting {
					c.Queue().Add(keyName)
				}
				return
			}
			runtimeObj, ok := obj.(runtime.Object)
			if !ok {
				utilruntime.HandleError(fmt.Errorf("added object %+v is not runtime Object", obj))
				return
			}
			if !isNamespace {
				c.Queue().Add(runtimeObj)
			} else if isInteresting {
				c.Queue().Add(runtimeObj)
			}
		},
		UpdateFunc: func(old, new interface{}) {
			isNamespace, isInteresting := c.isInterestingNamespace(new, interestingNamespaces)
			if !objectQueue {
				if !isNamespace {
					c.Queue().Add(keyName)
				} else if isInteresting {
					c.Queue().Add(keyName)
				}
				return
			}
			runtimeObj, ok := new.(runtime.Object)
			if !ok {
				utilruntime.HandleError(fmt.Errorf("updated object %+v is not runtime Object", runtimeObj))
				return
			}
			if !isNamespace {
				c.Queue().Add(runtimeObj)
			} else if isInteresting {
				c.Queue().Add(runtimeObj)
			}
		},
		DeleteFunc: func(obj interface{}) {
			isNamespace, isInteresting := c.isInterestingNamespace(obj, interestingNamespaces)
			if !objectQueue {
				if !isNamespace {
					c.Queue().Add(keyName)
				} else if isInteresting {
					c.Queue().Add(keyName)
				}
				return
			}
			runtimeObj, ok := obj.(runtime.Object)
			if !ok {
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if ok {
					if !isNamespace {
						c.Queue().Add(tombstone.Obj.(runtime.Object))
					} else if isInteresting {
						c.Queue().Add(tombstone.Obj.(runtime.Object))
					}
					return
				}
				utilruntime.HandleError(fmt.Errorf("updated object %+v is not runtime Object", runtimeObj))
				return
			}
			if !isNamespace {
				c.Queue().Add(runtimeObj)
			} else if isInteresting {
				c.Queue().Add(runtimeObj)
			}
		},
	}
}
