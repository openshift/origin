package authorizationsync

import (
	"fmt"
	"time"

	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/kubernetes/pkg/controller"
)

// genericController provides some boilerplate for the 4 (soon to be 8) controllers.
// They do very simple "listen, wait, work" flows.
type genericController struct {
	name         string
	cachesSynced cache.InformerSynced
	syncFunc     func(key string) error

	queue workqueue.RateLimitingInterface
}

func (c *genericController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()
	defer glog.Infof("Shutting %v controller", c.name)

	glog.Infof("Starting %v controller", c.name)

	if !cache.WaitForCacheSync(stopCh, c.cachesSynced) {
		utilruntime.HandleError(fmt.Errorf("%v: timed out waiting for caches to sync", c.name))
		return
	}

	for i := 0; i < workers; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	<-stopCh
}

func (c *genericController) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *genericController) processNextWorkItem() bool {
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(key)

	err := c.syncFunc(key.(string))
	if err == nil {
		c.queue.Forget(key)
		return true
	}

	utilruntime.HandleError(fmt.Errorf("%v: %v failed with : %v", c.name, key, err))
	c.queue.AddRateLimited(key)

	return true
}

func naiveEventHandler(queue workqueue.Interface) cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, err := controller.KeyFunc(obj)
			if err != nil {
				utilruntime.HandleError(err)
				return
			}
			queue.Add(key)
		},
		UpdateFunc: func(old, cur interface{}) {
			key, err := controller.KeyFunc(cur)
			if err != nil {
				utilruntime.HandleError(err)
				return
			}
			queue.Add(key)
		},
		DeleteFunc: func(obj interface{}) {
			key, err := getDeleteKey(obj)
			if err != nil {
				utilruntime.HandleError(err)
				return
			}
			queue.Add(key)
		},
	}
}

func getDeleteKey(uncast interface{}) (string, error) {
	obj, ok := uncast.(runtime.Object)
	if !ok {
		tombstone, ok := uncast.(cache.DeletedFinalStateUnknown)
		if !ok {
			return "", fmt.Errorf("Couldn't get object from tombstone %#v", uncast)
		}
		obj, ok = tombstone.Obj.(runtime.Object)
		if !ok {
			return "", fmt.Errorf("Tombstone contained object that is not a runtime.Object %#v", uncast)
		}
	}
	return controller.KeyFunc(obj)
}

var cloner = conversion.NewCloner()
