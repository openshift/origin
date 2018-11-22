package controller

import (
	"fmt"
	"time"

	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

type Runner interface {
	Run(workers int, stopCh <-chan struct{})
}

func New(name string, sync KeySyncer, opts ...Option) Runner {
	c := &controller{
		name: name,
		sync: sync,
	}

	WithRateLimiter(workqueue.DefaultControllerRateLimiter())(c)

	for _, opt := range opts {
		opt(c)
	}

	return c
}

type controller struct {
	name string
	sync KeySyncer

	queue      workqueue.RateLimitingInterface
	maxRetries int

	run     bool
	runOpts []Option

	cacheSyncs []cache.InformerSynced
}

func (c *controller) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	glog.Infof("Starting %s", c.name)
	defer glog.Infof("Shutting down %s", c.name)

	c.run = true
	for _, opt := range c.runOpts {
		opt(c)
	}

	if !cache.WaitForCacheSync(stopCh, c.cacheSyncs...) {
		utilruntime.HandleError(fmt.Errorf("%s: timed out waiting for caches to sync", c.name))
		return
	}

	for i := 0; i < workers; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	<-stopCh
}

func (c *controller) add(filter ParentFilter, object v1.Object) {
	namespace, name := filter.Parent(object)
	qKey := queueKey{namespace: namespace, name: name}
	c.queue.Add(qKey)
}

func (c *controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *controller) processNextWorkItem() bool {
	key, quit := c.queue.Get()
	if quit {
		return false
	}

	qKey := key.(queueKey)
	defer c.queue.Done(qKey)

	err := c.handleSync(qKey)
	c.handleKey(qKey, err)

	return true
}

func (c *controller) handleSync(key queueKey) error {
	obj, err := c.sync.Key(key.namespace, key.name)
	if errors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	return c.sync.Sync(obj)
}

func (c *controller) handleKey(key queueKey, err error) {
	if err == nil {
		c.queue.Forget(key)
		return
	}

	retryForever := c.maxRetries <= 0
	if retryForever || c.queue.NumRequeues(key) < c.maxRetries {
		utilruntime.HandleError(fmt.Errorf("%v failed with: %v", key, err))
		c.queue.AddRateLimited(key)
		return
	}

	utilruntime.HandleError(fmt.Errorf("dropping key %v out of the queue: %v", key, err))
	c.queue.Forget(key)
}

type queueKey struct {
	namespace string
	name      string
}
