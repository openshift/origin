package clusterquotamapping

import (
	"fmt"
	"time"

	"github.com/golang/glog"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	kcoreinformers "k8s.io/client-go/informers/core/v1"
	kcorelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kcoreinternalinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/internalversion/core/internalversion"
	kcoreinternallisters "k8s.io/kubernetes/pkg/client/listers/core/internalversion"
	"k8s.io/kubernetes/pkg/controller"

	quotaapi "github.com/openshift/origin/pkg/quota/apis/quota"
	quotainformer "github.com/openshift/origin/pkg/quota/generated/informers/internalversion/quota/internalversion"
	quotalister "github.com/openshift/origin/pkg/quota/generated/listers/quota/internalversion"
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
// When going from quota to namespace, you can get back a namespace that doesn't exist.  There are no resource in a non-existence
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
func NewClusterQuotaMappingController(namespaceInformer kcoreinformers.NamespaceInformer, quotaInformer quotainformer.ClusterResourceQuotaInformer) *ClusterQuotaMappingController {
	c := newClusterQuotaMappingController(namespaceInformer.Informer(), quotaInformer)
	c.namespaceLister = v1NamespaceLister{lister: namespaceInformer.Lister()}
	return c
}

// NewClusterQuotaMappingControllerInternal provides an adapter for listing internal resources. This
// method may be removed in the future.
func NewClusterQuotaMappingControllerInternal(namespaceInformer kcoreinternalinformers.NamespaceInformer, quotaInformer quotainformer.ClusterResourceQuotaInformer) *ClusterQuotaMappingController {
	c := newClusterQuotaMappingController(namespaceInformer.Informer(), quotaInformer)
	c.namespaceLister = internalNamespaceLister{lister: namespaceInformer.Lister()}
	return c
}

type namespaceLister interface {
	Each(label labels.Selector, fn func(metav1.Object) bool) error
	Get(name string) (metav1.Object, error)
}

type v1NamespaceLister struct {
	lister kcorelisters.NamespaceLister
}

func (l v1NamespaceLister) Each(label labels.Selector, fn func(metav1.Object) bool) error {
	results, err := l.lister.List(label)
	if err != nil {
		return err
	}
	for i := range results {
		if !fn(results[i]) {
			return nil
		}
	}
	return nil
}
func (l v1NamespaceLister) Get(name string) (metav1.Object, error) {
	return l.lister.Get(name)
}

type internalNamespaceLister struct {
	lister kcoreinternallisters.NamespaceLister
}

func (l internalNamespaceLister) Each(label labels.Selector, fn func(metav1.Object) bool) error {
	results, err := l.lister.List(label)
	if err != nil {
		return err
	}
	for i := range results {
		if !fn(results[i]) {
			return nil
		}
	}
	return nil
}
func (l internalNamespaceLister) Get(name string) (metav1.Object, error) {
	return l.lister.Get(name)
}

func newClusterQuotaMappingController(namespaceInformer cache.SharedIndexInformer, quotaInformer quotainformer.ClusterResourceQuotaInformer) *ClusterQuotaMappingController {
	c := &ClusterQuotaMappingController{
		namespaceQueue:     workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "controller_clusterquotamappingcontroller_namespaces"),
		quotaQueue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "controller_clusterquotamappingcontroller_clusterquotas"),
		clusterQuotaMapper: NewClusterQuotaMapper(),
	}
	namespaceInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.addNamespace,
		UpdateFunc: c.updateNamespace,
		DeleteFunc: c.deleteNamespace,
	})
	c.namespacesSynced = namespaceInformer.HasSynced

	quotaInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
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
	namespaceLister  namespaceLister
	namespacesSynced func() bool

	quotaQueue   workqueue.RateLimitingInterface
	quotaLister  quotalister.ClusterResourceQuotaLister
	quotasSynced func() bool

	clusterQuotaMapper *clusterQuotaMapper
}

func (c *ClusterQuotaMappingController) GetClusterQuotaMapper() ClusterQuotaMapper {
	return c.clusterQuotaMapper
}

func (c *ClusterQuotaMappingController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.namespaceQueue.ShutDown()
	defer c.quotaQueue.ShutDown()

	glog.Infof("Starting ClusterQuotaMappingController controller")
	defer glog.Infof("Shutting down ClusterQuotaMappingController controller")

	if !cache.WaitForCacheSync(stopCh, c.namespacesSynced, c.quotasSynced) {
		utilruntime.HandleError(fmt.Errorf("timed out waiting for caches to sync"))
		return
	}

	glog.V(4).Infof("Starting workers for quota mapping controller workers")
	for i := 0; i < workers; i++ {
		go wait.Until(c.namespaceWorker, time.Second, stopCh)
		go wait.Until(c.quotaWorker, time.Second, stopCh)
	}

	<-stopCh
}

func (c *ClusterQuotaMappingController) syncQuota(quota *quotaapi.ClusterResourceQuota) error {
	matcherFunc, err := quotaapi.GetObjectMatcher(quota.Spec.Selector)
	if err != nil {
		return err
	}

	if err := c.namespaceLister.Each(labels.Everything(), func(obj metav1.Object) bool {
		// attempt to set the mapping. The quotas never collide with each other (same quota is never processed twice in parallel)
		// so this means that the project we have is out of date, pull a more recent copy from the cache and retest
		for {
			matches, err := matcherFunc(obj)
			if err != nil {
				utilruntime.HandleError(err)
				break
			}
			success, quotaMatches, _ := c.clusterQuotaMapper.setMapping(quota, obj, !matches)
			if success {
				break
			}

			// if the quota is mismatched, then someone has updated the quota or has deleted the entry entirely.
			// if we've been updated, we'll be rekicked, if we've been deleted we should stop.  Either way, this
			// execution is finished
			if !quotaMatches {
				return false
			}
			newer, err := c.namespaceLister.Get(obj.GetName())
			if kapierrors.IsNotFound(err) {
				// if the namespace is gone, then the deleteNamespace path will be called, just continue
				break
			}
			if err != nil {
				utilruntime.HandleError(err)
				break
			}
			obj = newer
		}
		return true
	}); err != nil {
		return err
	}

	c.clusterQuotaMapper.completeQuota(quota)
	return nil
}

func (c *ClusterQuotaMappingController) syncNamespace(namespace metav1.Object) error {
	allQuotas, err1 := c.quotaLister.List(labels.Everything())
	if err1 != nil {
		return err1
	}
	for i := range allQuotas {
		quota := allQuotas[i]

		for {
			matcherFunc, err := quotaapi.GetObjectMatcher(quota.Spec.Selector)
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

	quota, err := c.quotaLister.Get(key.(string))
	if err != nil {
		if errors.IsNotFound(err) {
			c.quotaQueue.Forget(key)
			return false
		}
		utilruntime.HandleError(err)
		return false
	}

	err = c.syncQuota(quota)
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

	namespace, err := c.namespaceLister.Get(key.(string))
	if kapierrors.IsNotFound(err) {
		c.namespaceQueue.Forget(key)
		return false
	}
	if err != nil {
		utilruntime.HandleError(err)
		return false
	}

	err = c.syncNamespace(namespace)
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

func (c *ClusterQuotaMappingController) deleteNamespace(obj interface{}) {
	var name string
	switch ns := obj.(type) {
	case cache.DeletedFinalStateUnknown:
		switch nested := ns.Obj.(type) {
		case *v1.Namespace:
			name = nested.Name
		case *kapi.Namespace:
			name = nested.Name
		default:
			utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a Namespace %T", ns.Obj))
			return
		}
	case *v1.Namespace:
		name = ns.Name
	case *kapi.Namespace:
		name = ns.Name
	default:
		utilruntime.HandleError(fmt.Errorf("not a Namespace %v", obj))
		return
	}
	c.clusterQuotaMapper.removeNamespace(name)
}

func (c *ClusterQuotaMappingController) addNamespace(cur interface{}) {
	c.enqueueNamespace(cur)
}
func (c *ClusterQuotaMappingController) updateNamespace(old, cur interface{}) {
	c.enqueueNamespace(cur)
}
func (c *ClusterQuotaMappingController) enqueueNamespace(obj interface{}) {
	switch ns := obj.(type) {
	case *v1.Namespace:
		if !c.clusterQuotaMapper.requireNamespace(ns) {
			return
		}
	case *kapi.Namespace:
		adaptedNs := &v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:        ns.Name,
				Labels:      ns.Labels,
				Annotations: ns.Annotations,
			},
		}
		if !c.clusterQuotaMapper.requireNamespace(adaptedNs) {
			return
		}
	default:
		utilruntime.HandleError(fmt.Errorf("not a Namespace %v", obj))
		return
	}

	key, err := controller.KeyFunc(obj)
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
