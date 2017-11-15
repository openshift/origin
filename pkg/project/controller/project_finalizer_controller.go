package controller

import (
	"fmt"
	"time"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	informers "k8s.io/client-go/informers/core/v1"
	kclientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"github.com/golang/glog"
	projectutil "github.com/openshift/origin/pkg/project/util"
)

// ProjectFinalizerController is responsible for participating in Kubernetes Namespace termination
type ProjectFinalizerController struct {
	client kclientset.Interface

	queue      workqueue.RateLimitingInterface
	maxRetries int

	controller cache.Controller
	cache      cache.Store

	// extracted for testing
	syncHandler func(key string) error
}

func NewProjectFinalizerController(namespaces informers.NamespaceInformer, client kclientset.Interface) *ProjectFinalizerController {
	c := &ProjectFinalizerController{
		client:     client,
		controller: namespaces.Informer().GetController(),
		cache:      namespaces.Informer().GetStore(),
		queue:      workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
		maxRetries: 10,
	}
	namespaces.Informer().AddEventHandlerWithResyncPeriod(
		// TODO: generalize naiveResourceEventHandler and use it here
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				c.enqueueNamespace(obj)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				c.enqueueNamespace(newObj)
			},
		},
		10*time.Minute,
	)

	c.syncHandler = c.syncNamespace
	return c
}

// Run starts the workers for this controller.
func (c *ProjectFinalizerController) Run(stopCh <-chan struct{}, workers int) {
	defer runtime.HandleCrash()
	defer c.queue.ShutDown()

	// Wait for the stores to fill
	if !cache.WaitForCacheSync(stopCh, c.controller.HasSynced) {
		return
	}

	glog.V(5).Infof("Starting workers")
	for i := 0; i < workers; i++ {
		go c.worker()
	}
	<-stopCh
	glog.V(1).Infof("Shutting down")
}

func (c *ProjectFinalizerController) enqueueNamespace(obj interface{}) {
	ns, ok := obj.(*v1.Namespace)
	if !ok {
		return
	}
	c.queue.Add(ns.Name)
}

// worker runs a worker thread that just dequeues items, processes them, and marks them done.
// It enforces that the syncHandler is never invoked concurrently with the same key.
func (c *ProjectFinalizerController) worker() {
	for {
		if !c.work() {
			return
		}
	}
}

// work returns true if the worker thread should continue
func (c *ProjectFinalizerController) work() bool {
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(key)

	if err := c.syncHandler(key.(string)); err == nil {
		// this means the request was successfully handled.  We should "forget" the item so that any retry
		// later on is reset
		c.queue.Forget(key)
	} else {
		// if we had an error it means that we didn't handle it, which means that we want to requeue the work
		runtime.HandleError(fmt.Errorf("error syncing namespace, it will be retried: %v", err))
		c.queue.AddRateLimited(key)
	}
	return true
}

// syncNamespace will sync the namespace with the given key.
// This function is not meant to be invoked concurrently with the same key.
func (c *ProjectFinalizerController) syncNamespace(key string) error {
	item, exists, err := c.cache.GetByKey(key)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	return c.finalize(item.(*v1.Namespace))
}

// finalize processes a namespace and deletes content in origin if its terminating
func (c *ProjectFinalizerController) finalize(namespace *v1.Namespace) error {
	// if namespace is not terminating, ignore it
	if namespace.Status.Phase != v1.NamespaceTerminating {
		return nil
	}

	// if we already processed this namespace, ignore it
	if projectutil.Finalized(namespace) {
		return nil
	}

	// we have removed content, so mark it finalized by us
	_, err := projectutil.Finalize(c.client, namespace)
	return err
}
