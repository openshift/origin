package controller

import (
	"fmt"
	"time"

	"github.com/golang/glog"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

func New(name string, sync cache.ProcessFunc, cacheSyncs ...cache.InformerSynced) (*Controller, workqueue.RateLimitingInterface) {
	c := &Controller{
		name:        name,
		syncHandler: sync,
		cacheSyncs:  cacheSyncs,
		queue:       workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), name),
	}
	return c, c.queue
}

type Controller struct {
	name        string
	syncHandler cache.ProcessFunc
	cacheSyncs  []cache.InformerSynced
	queue       workqueue.RateLimitingInterface
}

// Run starts the serviceCertSigner and blocks until stopCh is closed.
func (c *Controller) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	glog.Infof("Starting %s", c.name)
	defer glog.Infof("Shutting down %s", c.name)

	if !cache.WaitForCacheSync(stopCh, c.cacheSyncs...) {
		utilruntime.HandleError(fmt.Errorf("timed out waiting for caches to sync"))
		return
	}

	for i := 0; i < workers; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	<-stopCh
}

func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *Controller) processNextWorkItem() bool {
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(key)

	err := c.syncHandler(key)
	if err == nil {
		c.queue.Forget(key)
		return true
	}

	utilruntime.HandleError(fmt.Errorf("%v failed with: %v", key, err))
	c.queue.AddRateLimited(key)

	return true
}
