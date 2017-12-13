package clusterquotareconciliation

import (
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/api/equality"
	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kutilerrors "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/controller"
	"k8s.io/kubernetes/pkg/controller/resourcequota"
	utilquota "k8s.io/kubernetes/pkg/quota"

	quotaapi "github.com/openshift/origin/pkg/quota/apis/quota"
	"github.com/openshift/origin/pkg/quota/controller/clusterquotamapping"
	quotainformer "github.com/openshift/origin/pkg/quota/generated/informers/internalversion/quota/internalversion"
	quotatypedclient "github.com/openshift/origin/pkg/quota/generated/internalclientset/typed/quota/internalversion"
	quotalister "github.com/openshift/origin/pkg/quota/generated/listers/quota/internalversion"
)

type ClusterQuotaReconcilationControllerOptions struct {
	ClusterQuotaInformer quotainformer.ClusterResourceQuotaInformer
	ClusterQuotaMapper   clusterquotamapping.ClusterQuotaMapper
	ClusterQuotaClient   quotatypedclient.ClusterResourceQuotaInterface

	// Knows how to calculate usage
	Registry utilquota.Registry
	// Controls full recalculation of quota usage
	ResyncPeriod time.Duration
	// Discover list of supported resources on the server.
	DiscoveryFunc resourcequota.NamespacedResourcesFunc
	// A function that returns the list of resources to ignore
	IgnoredResourcesFunc func() map[schema.GroupResource]struct{}
	// InformersStarted knows if informers were started.
	InformersStarted <-chan struct{}
	// InformerFactory interfaces with informers.
	InformerFactory resourcequota.InformerFactory
	// Controls full resync of objects monitored for replenihsment.
	ReplenishmentResyncPeriod controller.ResyncPeriodFunc
}

type ClusterQuotaReconcilationController struct {
	clusterQuotaLister quotalister.ClusterResourceQuotaLister
	clusterQuotaMapper clusterquotamapping.ClusterQuotaMapper
	clusterQuotaClient quotatypedclient.ClusterResourceQuotaInterface
	// A list of functions that return true when their caches have synced
	informerSyncedFuncs []cache.InformerSynced

	resyncPeriod time.Duration

	// queue tracks which clusterquotas to update along with a list of namespaces for that clusterquota
	queue BucketingWorkQueue

	// knows how to calculate usage
	registry utilquota.Registry
	// knows how to monitor all the resources tracked by quota and trigger replenishment
	quotaMonitor *resourcequota.QuotaMonitor
	// controls the workers that process quotas
	// this lock is acquired to control write access to the monitors and ensures that all
	// monitors are synced before the controller can process quotas.
	workerLock sync.RWMutex
}

type workItem struct {
	namespaceName      string
	forceRecalculation bool
}

func NewClusterQuotaReconcilationController(options ClusterQuotaReconcilationControllerOptions) (*ClusterQuotaReconcilationController, error) {
	c := &ClusterQuotaReconcilationController{
		clusterQuotaLister:  options.ClusterQuotaInformer.Lister(),
		clusterQuotaMapper:  options.ClusterQuotaMapper,
		clusterQuotaClient:  options.ClusterQuotaClient,
		informerSyncedFuncs: []cache.InformerSynced{options.ClusterQuotaInformer.Informer().HasSynced},

		resyncPeriod: options.ResyncPeriod,
		registry:     options.Registry,

		queue: NewBucketingWorkQueue("controller_clusterquotareconcilationcontroller"),
	}

	// we need to trigger every time
	options.ClusterQuotaInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.addClusterQuota,
		UpdateFunc: c.updateClusterQuota,
	})

	qm := resourcequota.NewQuotaMonitor(
		options.InformersStarted,
		options.InformerFactory,
		options.IgnoredResourcesFunc(),
		options.ReplenishmentResyncPeriod,
		c.replenishQuota,
		c.registry,
	)

	c.quotaMonitor = qm

	// do initial quota monitor setup
	resources, err := resourcequota.GetQuotableResources(options.DiscoveryFunc)
	if err != nil {
		return nil, err
	}
	if err = qm.SyncMonitors(resources); err != nil {
		utilruntime.HandleError(fmt.Errorf("initial monitor sync has error: %v", err))
	}

	// only start quota once all informers synced
	c.informerSyncedFuncs = append(c.informerSyncedFuncs, qm.IsSynced)

	return c, nil
}

// Run begins quota controller using the specified number of workers
func (c *ClusterQuotaReconcilationController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()

	glog.Infof("Starting the cluster quota reconciliation controller")

	// the controllers that replenish other resources to respond rapidly to state changes
	go c.quotaMonitor.Run(stopCh)

	if !controller.WaitForCacheSync("cluster resource quota", stopCh, c.informerSyncedFuncs...) {
		return
	}

	// the workers that chug through the quota calculation backlog
	for i := 0; i < workers; i++ {
		go wait.Until(c.worker, time.Second, stopCh)
	}

	// the timer for how often we do a full recalculation across all quotas
	go wait.Until(func() { c.calculateAll() }, c.resyncPeriod, stopCh)

	<-stopCh
	glog.Infof("Shutting down ClusterQuotaReconcilationController")
	c.queue.ShutDown()
}

// Sync periodically resyncs the controller when new resources are observed from discovery.
func (c *ClusterQuotaReconcilationController) Sync(discoveryFunc resourcequota.NamespacedResourcesFunc, period time.Duration, stopCh <-chan struct{}) {
	// Something has changed, so track the new state and perform a sync.
	oldResources := make(map[schema.GroupVersionResource]struct{})
	wait.Until(func() {
		// Get the current resource list from discovery.
		newResources, err := resourcequota.GetQuotableResources(discoveryFunc)
		if err != nil {
			utilruntime.HandleError(err)
			return
		}

		// Decide whether discovery has reported a change.
		if reflect.DeepEqual(oldResources, newResources) {
			glog.V(4).Infof("no resource updates from discovery, skipping resource quota sync")
			return
		}

		// Something has changed, so track the new state and perform a sync.
		glog.V(2).Infof("syncing resource quota controller with updated resources from discovery: %v", newResources)
		oldResources = newResources

		// Ensure workers are paused to avoid processing events before informers
		// have resynced.
		c.workerLock.Lock()
		defer c.workerLock.Unlock()

		// Perform the monitor resync and wait for controllers to report cache sync.
		if err := c.resyncMonitors(newResources); err != nil {
			utilruntime.HandleError(fmt.Errorf("failed to sync resource monitors: %v", err))
			return
		}
		if c.quotaMonitor != nil && !controller.WaitForCacheSync("cluster resource quota", stopCh, c.quotaMonitor.IsSynced) {
			utilruntime.HandleError(fmt.Errorf("timed out waiting for quota monitor sync"))
		}
	}, period, stopCh)
}

// resyncMonitors starts or stops quota monitors as needed to ensure that all
// (and only) those resources present in the map are monitored.
func (c *ClusterQuotaReconcilationController) resyncMonitors(resources map[schema.GroupVersionResource]struct{}) error {
	if err := c.quotaMonitor.SyncMonitors(resources); err != nil {
		return err
	}
	c.quotaMonitor.StartMonitors()
	return nil
}

func (c *ClusterQuotaReconcilationController) calculate(quotaName string, namespaceNames ...string) {
	if len(namespaceNames) == 0 {
		return
	}
	items := make([]interface{}, 0, len(namespaceNames))
	for _, name := range namespaceNames {
		items = append(items, workItem{namespaceName: name, forceRecalculation: false})
	}

	c.queue.AddWithData(quotaName, items...)
}

func (c *ClusterQuotaReconcilationController) forceCalculation(quotaName string, namespaceNames ...string) {
	if len(namespaceNames) == 0 {
		return
	}
	items := make([]interface{}, 0, len(namespaceNames))
	for _, name := range namespaceNames {
		items = append(items, workItem{namespaceName: name, forceRecalculation: true})
	}

	c.queue.AddWithData(quotaName, items...)
}

func (c *ClusterQuotaReconcilationController) calculateAll() {
	quotas, err := c.clusterQuotaLister.List(labels.Everything())
	if err != nil {
		utilruntime.HandleError(err)
		return
	}

	for _, quota := range quotas {
		// If we have namespaces we map to, force calculating those namespaces
		namespaces, _ := c.clusterQuotaMapper.GetNamespacesFor(quota.Name)
		if len(namespaces) > 0 {
			c.forceCalculation(quota.Name, namespaces...)
			continue
		}

		// If the quota status has namespaces when our mapper doesn't think it should,
		// add it directly to the queue without any work items
		if quota.Status.Namespaces.OrderedKeys().Front() != nil {
			c.queue.AddWithData(quota.Name)
			continue
		}
	}
}

// worker runs a worker thread that just dequeues items, processes them, and marks them done.
// It enforces that the syncHandler is never invoked concurrently with the same key.
func (c *ClusterQuotaReconcilationController) worker() {
	workFunc := func() bool {
		c.workerLock.RLock()
		defer c.workerLock.RUnlock()

		uncastKey, uncastData, quit := c.queue.GetWithData()
		if quit {
			return true
		}
		defer c.queue.Done(uncastKey)

		quotaName := uncastKey.(string)
		quota, err := c.clusterQuotaLister.Get(quotaName)
		if kapierrors.IsNotFound(err) {
			c.queue.Forget(uncastKey)
			return false
		}
		if err != nil {
			utilruntime.HandleError(err)
			c.queue.AddWithDataRateLimited(uncastKey, uncastData...)
			return false
		}

		workItems := make([]workItem, 0, len(uncastData))
		for _, dataElement := range uncastData {
			workItems = append(workItems, dataElement.(workItem))
		}
		err, retryItems := c.syncQuotaForNamespaces(quota, workItems)
		if err == nil {
			c.queue.Forget(uncastKey)
			return false
		}
		utilruntime.HandleError(err)

		items := make([]interface{}, 0, len(retryItems))
		for _, item := range retryItems {
			items = append(items, item)
		}
		c.queue.AddWithDataRateLimited(uncastKey, items...)
		return false
	}

	for {
		if quit := workFunc(); quit {
			glog.Infof("resource quota controller worker shutting down")
			return
		}
	}
}

// syncResourceQuotaFromKey syncs a quota key
func (c *ClusterQuotaReconcilationController) syncQuotaForNamespaces(originalQuota *quotaapi.ClusterResourceQuota, workItems []workItem) (error, []workItem /* to retry */) {
	quota := originalQuota.DeepCopy()

	// get the list of namespaces that match this cluster quota
	matchingNamespaceNamesList, quotaSelector := c.clusterQuotaMapper.GetNamespacesFor(quota.Name)
	if !equality.Semantic.DeepEqual(quotaSelector, quota.Spec.Selector) {
		return fmt.Errorf("mapping not up to date, have=%v need=%v", quotaSelector, quota.Spec.Selector), workItems
	}
	matchingNamespaceNames := sets.NewString(matchingNamespaceNamesList...)

	reconcilationErrors := []error{}
	retryItems := []workItem{}
	for _, item := range workItems {
		namespaceName := item.namespaceName
		namespaceTotals, namespaceLoaded := quota.Status.Namespaces.Get(namespaceName)
		if !matchingNamespaceNames.Has(namespaceName) {
			if namespaceLoaded {
				// remove this item from all totals
				quota.Status.Total.Used = utilquota.Subtract(quota.Status.Total.Used, namespaceTotals.Used)
				quota.Status.Namespaces.Remove(namespaceName)
			}
			continue
		}

		// if there's no work for us to do, do nothing
		if !item.forceRecalculation && namespaceLoaded && equality.Semantic.DeepEqual(namespaceTotals.Hard, quota.Spec.Quota.Hard) {
			continue
		}

		actualUsage, err := quotaUsageCalculationFunc(namespaceName, quota.Spec.Quota.Scopes, quota.Spec.Quota.Hard, c.registry)
		if err != nil {
			// tally up errors, but calculate everything you can
			reconcilationErrors = append(reconcilationErrors, err)
			retryItems = append(retryItems, item)
			continue
		}
		recalculatedStatus := kapi.ResourceQuotaStatus{
			Used: actualUsage,
			Hard: quota.Spec.Quota.Hard,
		}

		// subtract old usage, add new usage
		quota.Status.Total.Used = utilquota.Subtract(quota.Status.Total.Used, namespaceTotals.Used)
		quota.Status.Total.Used = utilquota.Add(quota.Status.Total.Used, recalculatedStatus.Used)
		quota.Status.Namespaces.Insert(namespaceName, recalculatedStatus)
	}

	// Remove any namespaces from quota.status that no longer match.
	// Needed because we will never get workitems for namespaces that no longer exist if we missed the delete event (e.g. on startup)
	for e := quota.Status.Namespaces.OrderedKeys().Front(); e != nil; e = e.Next() {
		namespaceName := e.Value.(string)
		namespaceTotals, _ := quota.Status.Namespaces.Get(namespaceName)
		if !matchingNamespaceNames.Has(namespaceName) {
			quota.Status.Total.Used = utilquota.Subtract(quota.Status.Total.Used, namespaceTotals.Used)
			quota.Status.Namespaces.Remove(namespaceName)
		}
	}

	quota.Status.Total.Hard = quota.Spec.Quota.Hard

	// if there's no change, no update, return early.  NewAggregate returns nil on empty input
	if equality.Semantic.DeepEqual(quota, originalQuota) {
		return kutilerrors.NewAggregate(reconcilationErrors), retryItems
	}

	if _, err := c.clusterQuotaClient.UpdateStatus(quota); err != nil {
		return kutilerrors.NewAggregate(append(reconcilationErrors, err)), workItems
	}

	return kutilerrors.NewAggregate(reconcilationErrors), retryItems
}

// replenishQuota is a replenishment function invoked by a controller to notify that a quota should be recalculated
func (c *ClusterQuotaReconcilationController) replenishQuota(groupResource schema.GroupResource, namespace string) {
	// check if the quota controller can evaluate this kind, if not, ignore it altogether...
	releventEvaluators := []utilquota.Evaluator{}
	evaluators := c.registry.List()
	for i := range evaluators {
		evaluator := evaluators[i]
		if evaluator.GroupResource() == groupResource {
			releventEvaluators = append(releventEvaluators, evaluator)
		}
	}
	if len(releventEvaluators) == 0 {
		return
	}

	quotaNames, _ := c.clusterQuotaMapper.GetClusterQuotasFor(namespace)

	// only queue those quotas that are tracking a resource associated with this kind.
	for _, quotaName := range quotaNames {
		quota, err := c.clusterQuotaLister.Get(quotaName)
		if err != nil {
			// replenishment will be delayed, but we'll get back around to it later if it matters
			continue
		}

		resourceQuotaResources := utilquota.ResourceNames(quota.Status.Total.Hard)
		for _, evaluator := range releventEvaluators {
			matchedResources := evaluator.MatchingResources(resourceQuotaResources)
			if len(matchedResources) > 0 {
				// TODO: make this support targeted replenishment to a specific kind, right now it does a full recalc on that quota.
				c.forceCalculation(quotaName, namespace)
				break
			}
		}
	}
}

func (c *ClusterQuotaReconcilationController) addClusterQuota(cur interface{}) {
	c.enqueueClusterQuota(cur)
}
func (c *ClusterQuotaReconcilationController) updateClusterQuota(old, cur interface{}) {
	c.enqueueClusterQuota(cur)
}
func (c *ClusterQuotaReconcilationController) enqueueClusterQuota(obj interface{}) {
	quota, ok := obj.(*quotaapi.ClusterResourceQuota)
	if !ok {
		utilruntime.HandleError(fmt.Errorf("not a ClusterResourceQuota %v", obj))
		return
	}

	namespaces, _ := c.clusterQuotaMapper.GetNamespacesFor(quota.Name)
	c.calculate(quota.Name, namespaces...)
}

func (c *ClusterQuotaReconcilationController) AddMapping(quotaName, namespaceName string) {
	c.calculate(quotaName, namespaceName)

}
func (c *ClusterQuotaReconcilationController) RemoveMapping(quotaName, namespaceName string) {
	c.calculate(quotaName, namespaceName)
}

// quotaUsageCalculationFunc is a function to calculate quota usage.  It is only configurable for easy unit testing
// NEVER CHANGE THIS OUTSIDE A TEST
var quotaUsageCalculationFunc = utilquota.CalculateUsage
