package clusterquotamapping

import (
	"fmt"
	"sync"
	"time"

	"github.com/golang/glog"

	kapi "k8s.io/kubernetes/pkg/api"
	kapierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/controller"
	"k8s.io/kubernetes/pkg/controller/framework"
	"k8s.io/kubernetes/pkg/labels"
	utilruntime "k8s.io/kubernetes/pkg/util/runtime"
	"k8s.io/kubernetes/pkg/util/sets"
	"k8s.io/kubernetes/pkg/util/wait"
	"k8s.io/kubernetes/pkg/util/workqueue"

	ocache "github.com/openshift/origin/pkg/client/cache"
	"github.com/openshift/origin/pkg/controller/shared"
	quotaapi "github.com/openshift/origin/pkg/quota/api"
)

type ClusterQuotaMapper interface {
	// GetClusterQuotasFor returns the list of clusterquota names that this namespace matches.  It also
	// returns the labels associated with the namespace for the check so that callers can determine staleness
	GetClusterQuotasFor(namespaceName string) ([]string, map[string]string)
	// GetNamespacesFor returns the list of namespace names that this cluster quota matches.  It also
	// returns the selector associated with the clusterquota for the check so that callers can determine staleness
	GetNamespacesFor(quotaName string) ([]string, *unversioned.LabelSelector)
}

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
		namespaceQueue: workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),

		quotaQueue: workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),

		clusterQuotaMapper: &clusterQuotaMapper{
			requiredQuotaToSelector:    map[string]*unversioned.LabelSelector{},
			requiredNamespaceToLabels:  map[string]map[string]string{},
			completedQuotaToSelector:   map[string]*unversioned.LabelSelector{},
			completedNamespaceToLabels: map[string]map[string]string{},

			quotaToNamespaces: map[string]sets.String{},
			namespaceToQuota:  map[string]sets.String{},
		},
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
	namespaceLister  *ocache.IndexerToNamespaceLister
	namespacesSynced func() bool

	quotaQueue   workqueue.RateLimitingInterface
	quotaLister  *ocache.IndexerToClusterResourceQuotaLister
	quotasSynced func() bool

	clusterQuotaMapper *clusterQuotaMapper
}

// clusterQuotaMapper gives thread safe access to the actual mappings that are being stored.
// Many method use a shareable read lock to check status followed by a non-shareable
// write lock which double checks the condition before proceding.  Since locks aren't escalatable
// you have to perform the recheck because someone could have beaten you in.
type clusterQuotaMapper struct {
	lock sync.RWMutex

	// requiredQuotaToSelector indicates the latest label selector this controller has observed for a quota
	requiredQuotaToSelector map[string]*unversioned.LabelSelector
	// requiredNamespaceToLabels indicates the latest labels this controller has observed for a namespace
	requiredNamespaceToLabels map[string]map[string]string
	// completedQuotaToSelector indicates the latest label selector this controller has scanned against namespaces
	completedQuotaToSelector map[string]*unversioned.LabelSelector
	// completedNamespaceToLabels indicates the latest labels this controller has scanned against cluster quotas
	completedNamespaceToLabels map[string]map[string]string

	quotaToNamespaces map[string]sets.String
	namespaceToQuota  map[string]sets.String
}

func (m *clusterQuotaMapper) GetClusterQuotasFor(namespaceName string) ([]string, map[string]string) {
	m.lock.RLock()
	defer m.lock.RUnlock()

	quotas, ok := m.namespaceToQuota[namespaceName]
	if !ok {
		return []string{}, m.completedNamespaceToLabels[namespaceName]
	}
	return quotas.List(), m.completedNamespaceToLabels[namespaceName]
}

func (m *clusterQuotaMapper) GetNamespacesFor(quotaName string) ([]string, *unversioned.LabelSelector) {
	m.lock.RLock()
	defer m.lock.RUnlock()

	namespaces, ok := m.quotaToNamespaces[quotaName]
	if !ok {
		return []string{}, m.completedQuotaToSelector[quotaName]
	}
	return namespaces.List(), m.completedQuotaToSelector[quotaName]
}

// requireQuota updates the selector requirements for the given quota.  This prevents stale updates to the mapping itself.
// returns true if a modification was made
func (m *clusterQuotaMapper) requireQuota(quota *quotaapi.ClusterResourceQuota) bool {
	m.lock.RLock()
	selector, exists := m.requiredQuotaToSelector[quota.Name]
	m.lock.RUnlock()

	if selectorMatches(selector, exists, quota) {
		return false
	}

	m.lock.Lock()
	defer m.lock.Unlock()
	selector, exists = m.requiredQuotaToSelector[quota.Name]
	if selectorMatches(selector, exists, quota) {
		return false
	}

	m.requiredQuotaToSelector[quota.Name] = quota.Spec.Selector
	return true
}

// completeQuota updates the latest selector used to generate the mappings for this quota.  The value is returned
// by the Get methods for the mapping so that callers can determine staleness
func (m *clusterQuotaMapper) completeQuota(quota *quotaapi.ClusterResourceQuota) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.completedQuotaToSelector[quota.Name] = quota.Spec.Selector
}

// removeQuota deletes a quota from all mappings
func (m *clusterQuotaMapper) removeQuota(quotaName string) {
	m.lock.Lock()
	defer m.lock.Unlock()

	delete(m.requiredQuotaToSelector, quotaName)
	delete(m.completedQuotaToSelector, quotaName)
	delete(m.quotaToNamespaces, quotaName)
	for _, quotas := range m.namespaceToQuota {
		quotas.Delete(quotaName)
	}
}

// requireNamespace updates the label requirements for the given namespace.  This prevents stale updates to the mapping itself.
// returns true if a modification was made
func (m *clusterQuotaMapper) requireNamespace(namespace *kapi.Namespace) bool {
	m.lock.RLock()
	labels, exists := m.requiredNamespaceToLabels[namespace.Name]
	m.lock.RUnlock()

	if labelsMatch(labels, exists, namespace) {
		return false
	}

	m.lock.Lock()
	defer m.lock.Unlock()
	labels, exists = m.requiredNamespaceToLabels[namespace.Name]
	if labelsMatch(labels, exists, namespace) {
		return false
	}

	m.requiredNamespaceToLabels[namespace.Name] = namespace.Labels
	return true
}

// completeNamespace updates the latest labels used to generate the mappings for this namespace.  The value is returned
// by the Get methods for the mapping so that callers can determine staleness
func (m *clusterQuotaMapper) completeNamespace(namespace *kapi.Namespace) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.completedNamespaceToLabels[namespace.Name] = namespace.Labels
}

// removeNamespace deletes a namespace from all mappings
func (m *clusterQuotaMapper) removeNamespace(namespaceName string) {
	m.lock.Lock()
	defer m.lock.Unlock()

	delete(m.requiredNamespaceToLabels, namespaceName)
	delete(m.completedNamespaceToLabels, namespaceName)
	delete(m.namespaceToQuota, namespaceName)
	for _, namespaces := range m.quotaToNamespaces {
		namespaces.Delete(namespaceName)
	}
}

func selectorMatches(selector *unversioned.LabelSelector, exists bool, quota *quotaapi.ClusterResourceQuota) bool {
	return exists && kapi.Semantic.DeepEqual(selector, quota.Spec.Selector)
}
func labelsMatch(labels map[string]string, exists bool, namespace *kapi.Namespace) bool {
	return exists && kapi.Semantic.DeepEqual(labels, namespace.Labels)
}

// setMapping maps (or removes a mapping) between a clusterquota and a namespace
// It returns whether the action worked, whether the quota is out of date, whether the namespace is out of date
// This allows callers to decide whether to pull new information from the cache or simply skip execution
func (m *clusterQuotaMapper) setMapping(quota *quotaapi.ClusterResourceQuota, namespace *kapi.Namespace, remove bool) (bool /*added*/, bool /*quota matches*/, bool /*namespace matches*/) {
	m.lock.RLock()
	selector, selectorExists := m.requiredQuotaToSelector[quota.Name]
	labels, labelsExist := m.requiredNamespaceToLabels[namespace.Name]
	m.lock.RUnlock()

	if !selectorMatches(selector, selectorExists, quota) {
		return false, false, true
	}
	if !labelsMatch(labels, labelsExist, namespace) {
		return false, true, false
	}

	m.lock.Lock()
	defer m.lock.Unlock()
	selector, selectorExists = m.requiredQuotaToSelector[quota.Name]
	labels, labelsExist = m.requiredNamespaceToLabels[namespace.Name]
	if !selectorMatches(selector, selectorExists, quota) {
		return false, false, true
	}
	if !labelsMatch(labels, labelsExist, namespace) {
		return false, true, false
	}

	if remove {
		namespaces, ok := m.quotaToNamespaces[quota.Name]
		if !ok {
			m.quotaToNamespaces[quota.Name] = sets.String{}
		} else {
			namespaces.Delete(namespace.Name)
		}

		quotas, ok := m.namespaceToQuota[namespace.Name]
		if !ok {
			m.namespaceToQuota[namespace.Name] = sets.String{}
		} else {
			quotas.Delete(quota.Name)
		}

		return true, true, true
	}

	namespaces, ok := m.quotaToNamespaces[quota.Name]
	if !ok {
		m.quotaToNamespaces[quota.Name] = sets.NewString(namespace.Name)
	} else {
		namespaces.Insert(namespace.Name)
	}

	quotas, ok := m.namespaceToQuota[namespace.Name]
	if !ok {
		m.namespaceToQuota[namespace.Name] = sets.NewString(quota.Name)
	} else {
		quotas.Insert(quota.Name)
	}

	return true, true, true

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
	selector, err := unversioned.LabelSelectorAsSelector(quota.Spec.Selector)
	if err != nil {
		return err
	}

	allNamespaces, err := c.namespaceLister.List(kapi.ListOptions{})
	if err != nil {
		return err
	}
	for i := range allNamespaces {
		namespace := allNamespaces[i]

		// attempt to set the mapping. The quotas never collide with each other (same quota is never processed twice in parallel)
		// so this means that the project we have is out of date, pull a more recent copy from the cache and retest
		for {
			matches := namespace != nil
			if namespace != nil {
				matches = selector.Matches(labels.Set(namespace.Labels))
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
			namespace, err = c.namespaceLister.Get(namespace.Name)
			if kapierrors.IsNotFound(err) {
				// if the namespace is gone, then the deleteNamespace path will be called, just continue
				break
			}
			if err != nil {
				utilruntime.HandleError(err)
				break
			}
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
			selector, err := unversioned.LabelSelectorAsSelector(quota.Spec.Selector)
			if err != nil {
				utilruntime.HandleError(err)
				break
			}

			// attempt to set the mapping. The namespaces never collide with each other (same namespace is never processed twice in parallel)
			// so this means that the quota we have is out of date, pull a more recent copy from the cache and retest
			matches := namespace != nil
			if namespace != nil {
				matches = selector.Matches(labels.Set(namespace.Labels))
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
