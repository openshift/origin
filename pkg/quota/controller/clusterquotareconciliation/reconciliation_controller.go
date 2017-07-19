package clusterquotareconciliation

import (
	"fmt"
	"time"

	"github.com/golang/glog"

	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kutilerrors "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	kapi "k8s.io/kubernetes/pkg/api"
	kapihelper "k8s.io/kubernetes/pkg/api/helper"
	"k8s.io/kubernetes/pkg/controller"
	"k8s.io/kubernetes/pkg/controller/resourcequota"
	utilquota "k8s.io/kubernetes/pkg/quota"

	"github.com/openshift/origin/pkg/client"
	quotaapi "github.com/openshift/origin/pkg/quota/apis/quota"
	"github.com/openshift/origin/pkg/quota/controller/clusterquotamapping"
	quotainformer "github.com/openshift/origin/pkg/quota/generated/informers/internalversion/quota/internalversion"
	quotalister "github.com/openshift/origin/pkg/quota/generated/listers/quota/internalversion"
)

type ClusterQuotaReconcilationControllerOptions struct {
	ClusterQuotaInformer quotainformer.ClusterResourceQuotaInformer
	ClusterQuotaMapper   clusterquotamapping.ClusterQuotaMapper
	ClusterQuotaClient   client.ClusterResourceQuotasInterface

	// Knows how to calculate usage
	Registry utilquota.Registry
	// Controls full recalculation of quota usage
	ResyncPeriod time.Duration
	// Knows how to build controllers that notify replenishment events
	ControllerFactory resourcequota.ReplenishmentControllerFactory
	// Controls full resync of objects monitored for replenihsment.
	ReplenishmentResyncPeriod controller.ResyncPeriodFunc
	// List of GroupKind objects that should be monitored for replenishment at
	// a faster frequency than the quota controller recalculation interval
	GroupKindsToReplenish []schema.GroupKind
}

type ClusterQuotaReconcilationController struct {
	clusterQuotaLister quotalister.ClusterResourceQuotaLister
	clusterQuotaSynced func() bool
	clusterQuotaMapper clusterquotamapping.ClusterQuotaMapper
	clusterQuotaClient client.ClusterResourceQuotasInterface

	resyncPeriod time.Duration

	// queue tracks which clusterquotas to update along with a list of namespaces for that clusterquota
	queue BucketingWorkQueue

	// knows how to calculate usage
	registry utilquota.Registry
	// controllers monitoring to notify for replenishment
	replenishmentControllers []cache.Controller
}

type workItem struct {
	namespaceName      string
	forceRecalculation bool
}

func NewClusterQuotaReconcilationController(options ClusterQuotaReconcilationControllerOptions) *ClusterQuotaReconcilationController {
	c := &ClusterQuotaReconcilationController{
		clusterQuotaLister: options.ClusterQuotaInformer.Lister(),
		clusterQuotaSynced: options.ClusterQuotaInformer.Informer().HasSynced,
		clusterQuotaMapper: options.ClusterQuotaMapper,
		clusterQuotaClient: options.ClusterQuotaClient,

		resyncPeriod: options.ResyncPeriod,
		registry:     options.Registry,

		queue: NewBucketingWorkQueue("controller_clusterquotareconcilationcontroller"),
	}

	// we need to trigger every time
	options.ClusterQuotaInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.addClusterQuota,
		UpdateFunc: c.updateClusterQuota,
	})

	for _, groupKindToReplenish := range options.GroupKindsToReplenish {
		controllerOptions := &resourcequota.ReplenishmentControllerOptions{
			GroupKind:         groupKindToReplenish,
			ResyncPeriod:      options.ReplenishmentResyncPeriod,
			ReplenishmentFunc: c.replenishQuota,
		}
		replenishmentController, err := options.ControllerFactory.NewController(controllerOptions)
		if err != nil {
			glog.Warningf("quota controller unable to replenish %s due to %v, changes only accounted during full resync", groupKindToReplenish, err)
		} else {
			c.replenishmentControllers = append(c.replenishmentControllers, replenishmentController)
		}
	}
	return c
}

// Run begins quota controller using the specified number of workers
func (c *ClusterQuotaReconcilationController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()

	// Wait for the stores to sync before starting any work in this controller.
	ready := make(chan struct{})
	go c.waitForSyncedStores(ready, stopCh)
	select {
	case <-ready:
	case <-stopCh:
		return
	}
	glog.V(4).Infof("Starting the cluster quota reconciliation controller workers")

	// the controllers that replenish other resources to respond rapidly to state changes
	for _, replenishmentController := range c.replenishmentControllers {
		go replenishmentController.Run(stopCh)
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

func (c *ClusterQuotaReconcilationController) waitForSyncedStores(ready chan<- struct{}, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()

	for !c.clusterQuotaSynced() {
		glog.V(4).Infof("Waiting for the caches to sync before starting the cluster quota reconciliation controller workers")
		select {
		case <-time.After(100 * time.Millisecond):
		case <-stopCh:
			return
		}
	}
	close(ready)
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
	obj, err := kapi.Scheme.Copy(originalQuota)
	if err != nil {
		return err, workItems
	}
	quota := obj.(*quotaapi.ClusterResourceQuota)

	// get the list of namespaces that match this cluster quota
	matchingNamespaceNamesList, quotaSelector := c.clusterQuotaMapper.GetNamespacesFor(quota.Name)
	if !kapihelper.Semantic.DeepEqual(quotaSelector, quota.Spec.Selector) {
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
		if !item.forceRecalculation && namespaceLoaded && kapihelper.Semantic.DeepEqual(namespaceTotals.Hard, quota.Spec.Quota.Hard) {
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
	if kapihelper.Semantic.DeepEqual(quota, originalQuota) {
		return kutilerrors.NewAggregate(reconcilationErrors), retryItems
	}

	if _, err := c.clusterQuotaClient.ClusterResourceQuotas().UpdateStatus(quota); err != nil {
		return kutilerrors.NewAggregate(append(reconcilationErrors, err)), workItems
	}

	return kutilerrors.NewAggregate(reconcilationErrors), retryItems
}

// replenishQuota is a replenishment function invoked by a controller to notify that a quota should be recalculated
func (c *ClusterQuotaReconcilationController) replenishQuota(groupKind schema.GroupKind, namespace string, object runtime.Object) {
	// check if the quota controller can evaluate this kind, if not, ignore it altogether...
	evaluators := c.registry.Evaluators()
	evaluator, found := evaluators[groupKind]
	if !found {
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
		matchedResources := evaluator.MatchingResources(resourceQuotaResources)
		if len(matchedResources) > 0 {
			// TODO: make this support targeted replenishment to a specific kind, right now it does a full recalc on that quota.
			c.forceCalculation(quotaName, namespace)
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
