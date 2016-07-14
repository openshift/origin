package controller

import (
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/cache"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/flowcontrol"
	utilruntime "k8s.io/kubernetes/pkg/util/runtime"
	"k8s.io/kubernetes/pkg/watch"

	"github.com/openshift/origin/pkg/controller"
	"github.com/openshift/origin/pkg/security/uidallocator"
)

// AllocationFactory can create an Allocation controller.
type AllocationFactory struct {
	UIDAllocator uidallocator.Interface
	MCSAllocator MCSAllocationFunc
	Client       kclient.NamespaceInterface
	// Queue may be a FIFO queue of namespaces. If nil, will be initialized using
	// the client.
	Queue controller.ReQueue
}

// Create creates a Allocation.
func (f *AllocationFactory) Create() controller.RunnableController {
	if f.Queue == nil {
		lw := &cache.ListWatch{
			ListFunc: func(options kapi.ListOptions) (runtime.Object, error) {
				return f.Client.List(options)
			},
			WatchFunc: func(options kapi.ListOptions) (watch.Interface, error) {
				return f.Client.Watch(options)
			},
		}
		q := cache.NewResyncableFIFO(cache.MetaNamespaceKeyFunc)
		cache.NewReflector(lw, &kapi.Namespace{}, q, 10*time.Minute).Run()
		f.Queue = q
	}

	c := &Allocation{
		uid:    f.UIDAllocator,
		mcs:    f.MCSAllocator,
		client: f.Client,
	}

	return &controller.RetryController{
		Queue: f.Queue,
		RetryManager: controller.NewQueueRetryManager(
			f.Queue,
			cache.MetaNamespaceKeyFunc,
			func(obj interface{}, err error, retries controller.Retry) bool {
				utilruntime.HandleError(err)
				return retries.Count < 5
			},
			flowcontrol.NewTokenBucketRateLimiter(1, 10),
		),
		Handle: func(obj interface{}) error {
			r := obj.(*kapi.Namespace)
			return c.Next(r)
		},
	}
}
