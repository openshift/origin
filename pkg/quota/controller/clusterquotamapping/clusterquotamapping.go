package clusterquotamapping

import (
	"fmt"
	"time"

	"github.com/golang/glog"

	kapi "k8s.io/kubernetes/pkg/api"
	kapierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/controller"
	"k8s.io/kubernetes/pkg/controller/framework"
	"k8s.io/kubernetes/pkg/labels"
	utilruntime "k8s.io/kubernetes/pkg/util/runtime"
	"k8s.io/kubernetes/pkg/util/wait"
	"k8s.io/kubernetes/pkg/util/workqueue"

	ocache "github.com/openshift/origin/pkg/client/cache"
	"github.com/openshift/origin/pkg/controller/shared"
	quotaapi "github.com/openshift/origin/pkg/quota/api"
)

// Look out, here there be dragons!
// There is a race when dealing with the DeltaFifo compression used to back a reflector for a controller that uses two
// SharedInformers for both their watch events AND their caches.  The scenario looks like this
//
// 1. Add, Delete a namespace really fast, *before* the add is observed by the controller using the reflector.
// 2. Add or Update a quota that matches the Add namespace
// 3. The cache had the intermediate state for the namespace for some period of time.  This makes the quota update the mapping indicating a match.
// 4. The ns Delete is compressed out and never delivered to the controller, so the improper match is never cleared.
//
// This sounds pretty bad, however, we fail in the "safe" direction and the consequences are detectable.
// When going from quota to namespace, you can get back a namespace that doesn't exist.  There are no resource in a non-existance
// namespace, so you know to clear all referenced resources.  In addition, this add/delete has to happen so fast
// that it would be nearly impossible for any resources to be created.  If you do create resources, then we must be observing
// their deletes.  When quota is replenished, we'll see that we need to clear any charges.
//
// When going from namespace to quota, you can get back a quota that doesn't exist.  Since the cache is shared,
// we know that a missing quota means that there isn't anything for us to bill against, so we can skip it.
//
// If the mapping cache is wrong and a previously deleted quota or namespace is created, this controller
// correctly adds the items back to the list and clears out all previous mappings.
//
// In addition to those constraints, the timing threshold for actually hitting this problem is really tight.  It's
// basically a script that is creating and deleting things as fast as it possibly can.  Sub-millisecond in the fuzz
// test where I caught the problem.

// NewClusterQuotaMappingController builds a mapping between namespaces and clusterresourcequotas
func NewClusterQuotaMappingController(namespaceInformer shared.NamespaceInformer, quotaInformer shared.ClusterResourceQuotaInformer) *ClusterQuotaMappingController {
	c := &ClusterQuotaMappingController{
		namespaceQueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "controller_clusterquotamappingcontroller_namespaces"),

		quotaQueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "controller_clusterquotamappingcontroller_clusterquotas"),

		clusterQuotaMapper: NewClusterQuotaMapper(),
	}

	namespaceInformer.Informer().AddEventHandler(framework.ResourceEventHandlerFuncs{
		AddFunc:    c.addNamespace,
		UpdateFunc: c.updateNamespace,
		DeleteFunc: c.deleteNamespace,
	})
	c.namespaceLister = namespaceInformer.Lister()
	c.namespacesSynced = namespaceInformer.Informer().HasSynced

	quotaInformer.Informer().AddEventHandler(framework.ResourceEventHandlerFuncs{
		AddFunc:    c.addQuota,
		UpdateFunc: c.updateQuota,
		DeleteFunc: c.deleteQuota,
	})
	c.quotaLister = quotaInformer.Lister()
	c.quotasSynced = quotaInformer.Informer().HasSynced

	return c
}

type ClusterQuotaMappingController struct {
	namespaceQueue   workqueue.RateLimitingInterface
	namespaceLister  *cache.IndexerToNamespaceLister
	namespacesSynced func() bool

	quotaQueue   workqueue.RateLimitingInterface
	quotaLister  *ocache.IndexerToClusterResourceQuotaLister
	quotasSynced func() bool

	clusterQuotaMapper *clusterQuotaMapper
}

func (c *ClusterQuotaMappingController) GetClusterQuotaMapper() ClusterQuotaMapper {
	return c.clusterQuotaMapper
}

func (c *ClusterQuotaMappingController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()

	// Wait for the stores to sync before starting any work in this controller.
	ready := make(chan struct{})
	go c.waitForSyncedStores(ready, stopCh)
	select {
	case <-ready:
	case <-stopCh:
		return
	}

	glog.V(4).Infof("Starting workers for quota mapping controller workers")
	for i := 0; i < workers; i++ {
		go wait.Until(c.namespaceWorker, time.Second, stopCh)
		go wait.Until(c.quotaWorker, time.Second, stopCh)
	}

	<-stopCh
	glog.Infof("Shutting down quota mapping controller")
	c.namespaceQueue.ShutDown()
	c.quotaQueue.ShutDown()
}

func (c *ClusterQuotaMappingController) syncQuota(quota *quotaapi.ClusterResourceQuota) error {
	matcherFunc, err := quotaapi.GetMatcher(quota.Spec.Selector)
	if err != nil {
		return err
	}

	allNamespaces, err := c.namespaceLister.List(labels.Everything())
	if err != nil {
		return err
	}
	for i := range allNamespaces {
		namespace := allNamespaces[i]

		// attempt to set the mapping. The quotas never collide with each other (same quota is never processed twice in parallel)
		// so this means that the project we have is out of date, pull a more recent copy from the cache and retest
		for {
			matches, err := matcherFunc(namespace)
			if err != nil {
				utilruntime.HandleError(err)
				break
			}
			success, quotaMatches, _ := c.clusterQuotaMapper.setMapping(quota, namespace, !matches)
			if success {
				break
			}

			// if the quota is mismatched, then someone has updated the quota or has deleted the entry entirely.
			// if we've been updated, we'll be rekicked, if we've been deleted we should stop.  Either way, this
			// execution is finished
			if !quotaMatches {
				return nil
			}
			obj, ok, err := c.namespaceLister.Get(namespace.Name)
			if kapierrors.IsNotFound(err) || !ok {
				// if the namespace is gone, then the deleteNamespace path will be called, just continue
				break
			}
			if err != nil {
				utilruntime.HandleError(err)
				break
			}
			namespace = obj.(*kapi.Namespace)
		}

	}

	c.clusterQuotaMapper.completeQuota(quota)
	return nil
}

func (c *ClusterQuotaMappingController) syncNamespace(namespace *kapi.Namespace) error {
	allQuotas, err1 := c.quotaLister.List(kapi.ListOptions{})
	if err1 != nil {
		return err1
	}
	for i := range allQuotas {
		quota := allQuotas[i]

		for {
			matcherFunc, err := quotaapi.GetMatcher(quota.Spec.Selector)
			if err != nil {
				utilruntime.HandleError(err)
				break
			}

			// attempt to set the mapping. The namespaces never collide with each other (same namespace is never processed twice in parallel)
			// so this means that the quota we have is out of date, pull a more recent copy from the cache and retest
			matches, err := matcherFunc(namespace)
			if err != nil {
				utilruntime.HandleError(err)
				break
			}
			success, _, namespaceMatches := c.clusterQuotaMapper.setMapping(quota, namespace, !matches)
			if success {
				break
			}

			// if the namespace is mismatched, then someone has updated the namespace or has deleted the entry entirely.
			// if we've been updated, we'll be rekicked, if we've been deleted we should stop.  Either way, this
			// execution is finished
			if !namespaceMatches {
				return nil
			}

			quota, err = c.quotaLister.Get(quota.Name)
			if kapierrors.IsNotFound(err) {
				// if the quota is gone, then the deleteQuota path will be called, just continue
				break
			}
			if err != nil {
				utilruntime.HandleError(err)
				break
			}
		}
	}

	c.clusterQuotaMapper.completeNamespace(namespace)
	return nil
}

func (c *ClusterQuotaMappingController) quotaWork() bool {
	key, quit := c.quotaQueue.Get()
	if quit {
		return true
	}
	defer c.quotaQueue.Done(key)

	quota, exists, err := c.quotaLister.GetByKey(key.(string))
	if !exists {
		c.quotaQueue.Forget(key)
		return false
	}
	if err != nil {
		utilruntime.HandleError(err)
		return false
	}

	err = c.syncQuota(quota.(*quotaapi.ClusterResourceQuota))
	outOfRetries := c.quotaQueue.NumRequeues(key) > 5
	switch {
	case err != nil && outOfRetries:
		utilruntime.HandleError(err)
		c.quotaQueue.Forget(key)

	case err != nil && !outOfRetries:
		c.quotaQueue.AddRateLimited(key)

	default:
		c.quotaQueue.Forget(key)
	}

	return false
}

func (c *ClusterQuotaMappingController) quotaWorker() {
	for {
		if quit := c.quotaWork(); quit {
			return
		}
	}
}

func (c *ClusterQuotaMappingController) namespaceWork() bool {
	key, quit := c.namespaceQueue.Get()
	if quit {
		return true
	}
	defer c.namespaceQueue.Done(key)

	namespace, exists, err := c.namespaceLister.GetByKey(key.(string))
	if !exists {
		c.namespaceQueue.Forget(key)
		return false
	}
	if err != nil {
		utilruntime.HandleError(err)
		return false
	}

	err = c.syncNamespace(namespace.(*kapi.Namespace))
	outOfRetries := c.namespaceQueue.NumRequeues(key) > 5
	switch {
	case err != nil && outOfRetries:
		utilruntime.HandleError(err)
		c.namespaceQueue.Forget(key)

	case err != nil && !outOfRetries:
		c.namespaceQueue.AddRateLimited(key)

	default:
		c.namespaceQueue.Forget(key)
	}

	return false
}

func (c *ClusterQuotaMappingController) namespaceWorker() {
	for {
		if quit := c.namespaceWork(); quit {
			return
		}
	}
}

func (c *ClusterQuotaMappingController) waitForSyncedStores(ready chan<- struct{}, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()

	for !c.namespacesSynced() || !c.quotasSynced() {
		glog.V(4).Infof("Waiting for the caches to sync before starting the quota mapping controller workers")
		select {
		case <-time.After(100 * time.Millisecond):
		case <-stopCh:
			return
		}
	}
	close(ready)
}

func (c *ClusterQuotaMappingController) deleteNamespace(obj interface{}) {
	ns, ok1 := obj.(*kapi.Namespace)
	if !ok1 {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone %v", obj))
			return
		}
		ns, ok = tombstone.Obj.(*kapi.Namespace)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a Namespace %v", obj))
			return
		}
	}

	c.clusterQuotaMapper.removeNamespace(ns.Name)
}

func (c *ClusterQuotaMappingController) addNamespace(cur interface{}) {
	c.enqueueNamespace(cur)
}
func (c *ClusterQuotaMappingController) updateNamespace(old, cur interface{}) {
	c.enqueueNamespace(cur)
}
func (c *ClusterQuotaMappingController) enqueueNamespace(obj interface{}) {
	ns, ok := obj.(*kapi.Namespace)
	if !ok {
		utilruntime.HandleError(fmt.Errorf("not a Namespace %v", obj))
		return
	}
	if !c.clusterQuotaMapper.requireNamespace(ns) {
		return
	}

	key, err := controller.KeyFunc(ns)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.namespaceQueue.Add(key)
}

func (c *ClusterQuotaMappingController) deleteQuota(obj interface{}) {
	quota, ok1 := obj.(*quotaapi.ClusterResourceQuota)
	if !ok1 {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone %v", obj))
			return
		}
		quota, ok = tombstone.Obj.(*quotaapi.ClusterResourceQuota)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a Quota %v", obj))
			return
		}
	}

	c.clusterQuotaMapper.removeQuota(quota.Name)
}

func (c *ClusterQuotaMappingController) addQuota(cur interface{}) {
	c.enqueueQuota(cur)
}
func (c *ClusterQuotaMappingController) updateQuota(old, cur interface{}) {
	c.enqueueQuota(cur)
}
func (c *ClusterQuotaMappingController) enqueueQuota(obj interface{}) {
	quota, ok := obj.(*quotaapi.ClusterResourceQuota)
	if !ok {
		utilruntime.HandleError(fmt.Errorf("not a Quota %v", obj))
		return
	}
	if !c.clusterQuotaMapper.requireQuota(quota) {
		return
	}

	key, err := controller.KeyFunc(quota)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.quotaQueue.Add(key)
}
