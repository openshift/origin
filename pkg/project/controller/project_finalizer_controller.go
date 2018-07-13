package controller

import (
	"fmt"

	"github.com/golang/glog"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	corev1informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	projectapiv1 "github.com/openshift/api/project/v1"
)

// ProjectFinalizerController is responsible for participating in Kubernetes Namespace termination
type ProjectFinalizerController struct {
	client kubernetes.Interface

	queue workqueue.RateLimitingInterface

	cacheSynced cache.InformerSynced
	nsLister    corev1listers.NamespaceLister

	// extracted for testing
	syncHandler func(key string) error
}

func NewProjectFinalizerController(namespaces corev1informers.NamespaceInformer, client kubernetes.Interface) *ProjectFinalizerController {
	c := &ProjectFinalizerController{
		client:      client,
		cacheSynced: namespaces.Informer().HasSynced,
		queue:       workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
		nsLister:    namespaces.Lister(),
	}
	namespaces.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				c.enqueueNamespace(obj)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				c.enqueueNamespace(newObj)
			},
		},
	)

	c.syncHandler = c.syncNamespace
	return c
}

// Run starts the workers for this controller.
func (c *ProjectFinalizerController) Run(stopCh <-chan struct{}, workers int) {
	defer runtime.HandleCrash()
	defer c.queue.ShutDown()

	// Wait for the stores to fill
	if !cache.WaitForCacheSync(stopCh, c.cacheSynced) {
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
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.queue.Add(key)
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
	ns, err := c.nsLister.Get(key)
	if errors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}

	found := false
	for _, finalizerName := range ns.Spec.Finalizers {
		if projectapiv1.FinalizerOrigin == finalizerName {
			found = true
		}
	}
	if !found {
		return nil
	}

	return c.finalize(ns.DeepCopy())
}

// finalize processes a namespace and deletes content in origin if its terminating
func (c *ProjectFinalizerController) finalize(namespace *v1.Namespace) error {
	finalizerSet := sets.NewString()
	for i := range namespace.Spec.Finalizers {
		finalizerSet.Insert(string(namespace.Spec.Finalizers[i]))
	}
	finalizerSet.Delete(string(projectapiv1.FinalizerOrigin))

	namespace.Spec.Finalizers = make([]v1.FinalizerName, 0, len(finalizerSet))
	for _, value := range finalizerSet.List() {
		namespace.Spec.Finalizers = append(namespace.Spec.Finalizers, v1.FinalizerName(value))
	}

	// we have removed content, so mark it finalized by us
	_, err := c.client.Core().Namespaces().Finalize(namespace)
	return err
}
