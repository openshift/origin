package controller

import (
	"fmt"
	"time"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	informers "k8s.io/client-go/informers/core/v1"
	kcoreclient "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"github.com/golang/glog"
	"github.com/openshift/origin/pkg/security"
	"github.com/openshift/origin/pkg/security/mcs"
	"github.com/openshift/origin/pkg/security/uid"
	"github.com/openshift/origin/pkg/security/uidallocator"
)

// NamespaceSecurityDefaultsController allocates uids/labels for namespaces
type NamespaceSecurityDefaultsController struct {
	uidAllocator uidallocator.Interface
	mcsAllocator MCSAllocationFunc

	client kcoreclient.NamespaceInterface

	queue      workqueue.RateLimitingInterface
	maxRetries int

	controller cache.Controller
	cache      cache.Store

	// extracted for testing
	syncHandler func(key string) error
}

func NewNamespaceSecurityDefaultsController(namespaces informers.NamespaceInformer, client kcoreclient.NamespaceInterface, uid uidallocator.Interface, mcs MCSAllocationFunc) *NamespaceSecurityDefaultsController {
	c := &NamespaceSecurityDefaultsController{
		uidAllocator: uid,
		mcsAllocator: mcs,
		client:       client,
		controller:   namespaces.Informer().GetController(),
		cache:        namespaces.Informer().GetStore(),
		queue:        workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
		maxRetries:   10,
	}
	namespaces.Informer().AddEventHandlerWithResyncPeriod(
		cache.ResourceEventHandlerFuncs{
			AddFunc: c.enqueueNamespace,
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
func (c *NamespaceSecurityDefaultsController) Run(stopCh <-chan struct{}, workers int) {
	defer utilruntime.HandleCrash()
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

func (c *NamespaceSecurityDefaultsController) enqueueNamespace(obj interface{}) {
	ns, ok := obj.(*v1.Namespace)
	if !ok {
		return
	}
	c.queue.Add(ns.Name)
}

// worker runs a worker thread that just dequeues items, processes them, and marks them done.
// It enforces that the syncHandler is never invoked concurrently with the same key.
func (c *NamespaceSecurityDefaultsController) worker() {
	for c.work() {
	}
}

// work returns true if the worker thread should continue
func (c *NamespaceSecurityDefaultsController) work() bool {
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
		utilruntime.HandleError(fmt.Errorf("error syncing namespace, it will be retried: %v", err))
		c.queue.AddRateLimited(key)
	}
	return true
}

// syncNamespace will sync the namespace with the given key.
// This function is not meant to be invoked concurrently with the same key.
func (c *NamespaceSecurityDefaultsController) syncNamespace(key string) error {
	item, exists, err := c.cache.GetByKey(key)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	return c.allocate(item.(*v1.Namespace))
}

type MCSAllocationFunc func(uid.Block) *mcs.Label

// DefaultMCSAllocation returns a label from the MCS range that matches the offset
// within the overall range. blockSize must be a positive integer representing the
// number of labels to jump past in the category space (if 1, range == label, if 2
// each range will have two labels).
func DefaultMCSAllocation(from *uid.Range, to *mcs.Range, blockSize int) MCSAllocationFunc {
	return func(block uid.Block) *mcs.Label {
		ok, offset := from.Offset(block)
		if !ok {
			return nil
		}
		if blockSize > 0 {
			offset = offset * uint32(blockSize)
		}
		label, _ := to.LabelAt(uint64(offset))
		return label
	}
}

// Next processes a changed namespace and tries to allocate a uid range for it.  If it is
// successful, an mcs label corresponding to the relative position of the range is also
// set.
func (c *NamespaceSecurityDefaultsController) allocate(ns *v1.Namespace) error {
	tx := &tx{}
	defer tx.Rollback()

	if _, ok := ns.Annotations[security.UIDRangeAnnotation]; ok {
		return nil
	}

	nsCopy := ns.DeepCopy()

	if nsCopy.Annotations == nil {
		nsCopy.Annotations = make(map[string]string)
	}

	// do uid allocation
	block, err := c.uidAllocator.AllocateNext()
	if err != nil {
		return err
	}
	tx.Add(func() error { return c.uidAllocator.Release(block) })

	nsCopy.Annotations[security.UIDRangeAnnotation] = block.String()
	nsCopy.Annotations[security.SupplementalGroupsAnnotation] = block.String()
	if _, ok := nsCopy.Annotations[security.MCSAnnotation]; !ok {
		if label := c.mcsAllocator(block); label != nil {
			nsCopy.Annotations[security.MCSAnnotation] = label.String()
		}
	}

	_, err = c.client.Update(nsCopy)
	if err == nil {
		tx.Commit()
	}
	if errors.IsNotFound(err) {
		return nil
	}
	return err
}

func changedAndSetAnnotations(old, ns *v1.Namespace) bool {
	if value, ok := ns.Annotations[security.UIDRangeAnnotation]; ok && value != old.Annotations[security.UIDRangeAnnotation] {
		return true
	}
	if value, ok := ns.Annotations[security.MCSAnnotation]; ok && value != old.Annotations[security.MCSAnnotation] {
		return true
	}
	if value, ok := ns.Annotations[security.SupplementalGroupsAnnotation]; ok && value != old.Annotations[security.SupplementalGroupsAnnotation] {
		return true
	}
	return false
}

type tx struct {
	rollback []func() error
}

func (tx *tx) Add(fn func() error) {
	tx.rollback = append(tx.rollback, fn)
}

func (tx *tx) HasChanges() bool {
	return len(tx.rollback) > 0
}

func (tx *tx) Rollback() {
	for _, fn := range tx.rollback {
		if err := fn(); err != nil {
			utilruntime.HandleError(fmt.Errorf("unable to undo tx: %v", err))
		}
	}
}

func (tx *tx) Commit() {
	tx.rollback = nil
}
