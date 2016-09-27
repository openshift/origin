package ingressip

import (
	"fmt"
	"net"
	"sort"
	"sync"
	"time"

	"github.com/golang/glog"
	kapi "k8s.io/kubernetes/pkg/api"
	kerrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/client/record"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/controller"
	"k8s.io/kubernetes/pkg/controller/framework"
	"k8s.io/kubernetes/pkg/registry/service/allocator"
	"k8s.io/kubernetes/pkg/registry/service/ipallocator"
	"k8s.io/kubernetes/pkg/runtime"
	utilruntime "k8s.io/kubernetes/pkg/util/runtime"
	"k8s.io/kubernetes/pkg/util/sets"
	"k8s.io/kubernetes/pkg/util/wait"
	"k8s.io/kubernetes/pkg/util/workqueue"
	"k8s.io/kubernetes/pkg/watch"
)

const (
	// It's necessary to allocate for the initial sync in consistent
	// order rather than in the order received.  This requires waiting
	// until the initial sync has been processed, and to avoid a hot
	// loop, we'll wait this long between checks.
	SyncProcessedPollPeriod = 100 * time.Millisecond

	clientRetryCount    = 5
	clientRetryInterval = 5 * time.Second
	clientRetryFactor   = 1.1
)

// IngressIPController is responsible for allocating ingress ip
// addresses to Service objects of type LoadBalancer.
type IngressIPController struct {
	client kclient.ServicesNamespacer

	controller *framework.Controller

	maxRetries int

	// Tracks ip allocation for the configured range
	ipAllocator *ipallocator.Range
	// Tracks ip -> service key to allow detection of duplicate ip
	// allocations.
	allocationMap map[string]string

	// Tracks services requeued for allocation when the range is full.
	requeuedAllocations sets.String

	// Protects the transition between initial sync and regular processing
	lock  sync.Mutex
	cache cache.Store
	queue workqueue.RateLimitingInterface

	// recorder is used to record events.
	recorder record.EventRecorder

	// changeHandler does the work. It can be factored out for unit testing.
	changeHandler func(change *serviceChange) error
	// persistenceHandler persists service changes.  It can be factored out for unit testing
	persistenceHandler func(client kclient.ServicesNamespacer, service *kapi.Service, targetStatus bool) error
}

type serviceChange struct {
	key                string
	oldService         *kapi.Service
	requeuedAllocation bool
}

// NewIngressIPController creates a new IngressIPController.
// TODO this should accept a shared informer
func NewIngressIPController(kc kclient.Interface, ipNet *net.IPNet, resyncInterval time.Duration) *IngressIPController {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(kc.Events(""))
	recorder := eventBroadcaster.NewRecorder(kapi.EventSource{Component: "ingressip-controller"})

	ic := &IngressIPController{
		client:     kc,
		queue:      workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
		maxRetries: 10,
		recorder:   recorder,
	}

	ic.cache, ic.controller = framework.NewInformer(
		&cache.ListWatch{
			ListFunc: func(options kapi.ListOptions) (runtime.Object, error) {
				return ic.client.Services(kapi.NamespaceAll).List(options)
			},
			WatchFunc: func(options kapi.ListOptions) (watch.Interface, error) {
				return ic.client.Services(kapi.NamespaceAll).Watch(options)
			},
		},
		&kapi.Service{},
		resyncInterval,
		framework.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				service := obj.(*kapi.Service)
				glog.V(5).Infof("Adding service %s/%s", service.Namespace, service.Name)
				ic.enqueueChange(obj, nil)
			},
			UpdateFunc: func(old, cur interface{}) {
				service := cur.(*kapi.Service)
				glog.V(5).Infof("Updating service %s/%s", service.Namespace, service.Name)
				ic.enqueueChange(cur, old)
			},
			DeleteFunc: func(obj interface{}) {
				service := obj.(*kapi.Service)
				glog.V(5).Infof("Deleting service %s/%s", service.Namespace, service.Name)
				ic.enqueueChange(nil, obj)
			},
		},
	)

	ic.changeHandler = ic.processChange
	ic.persistenceHandler = persistService

	ic.ipAllocator = ipallocator.NewAllocatorCIDRRange(ipNet, func(max int, rangeSpec string) allocator.Interface {
		return allocator.NewAllocationMap(max, rangeSpec)
	})

	ic.allocationMap = make(map[string]string)
	ic.requeuedAllocations = sets.NewString()

	return ic
}

// enqueueChange transforms the old and new objects into a change
// object and queues it.  A lock is shared with processInitialSync to
// avoid enqueueing while the changes from the initial sync are being
// processed.
func (ic *IngressIPController) enqueueChange(new interface{}, old interface{}) {
	ic.lock.Lock()
	defer ic.lock.Unlock()

	change := &serviceChange{}

	if new != nil {
		// Queue the key needed to retrieve the lastest state from the
		// cache when the change is processed.
		key, err := controller.KeyFunc(new)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("Couldn't get key for object %+v: %v", new, err))
			return
		}
		change.key = key
	}

	if old != nil {
		change.oldService = old.(*kapi.Service)
	}

	ic.queue.Add(change)
}

// Run begins watching and syncing.
func (ic *IngressIPController) Run(stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	go ic.controller.Run(stopCh)

	glog.V(5).Infof("Waiting for the initial sync to be completed")
	for !ic.controller.HasSynced() {
		select {
		case <-time.After(SyncProcessedPollPeriod):
		case <-stopCh:
			return
		}
	}

	if !ic.processInitialSync() {
		return
	}

	glog.V(5).Infof("Starting normal worker")
	for {
		if !ic.work() {
			break
		}
	}

	glog.V(5).Infof("Shutting down ingress ip controller")
	ic.queue.ShutDown()
}

type serviceAge []*kapi.Service

func (s serviceAge) Len() int      { return len(s) }
func (s serviceAge) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s serviceAge) Less(i, j int) bool {
	if s[i].CreationTimestamp.Before(s[j].CreationTimestamp) {
		return true
	}
	return (s[i].CreationTimestamp == s[j].CreationTimestamp && s[i].UID < s[j].UID)
}

// processInitialSync processes the items queued by informer's initial sync.
// A lock is shared between this method and enqueueService to ensure
// that queue additions are blocked while the sync is being processed.
// Returns a boolean indication of whether processing should continue.
func (ic *IngressIPController) processInitialSync() bool {
	ic.lock.Lock()
	defer ic.lock.Unlock()

	glog.V(5).Infof("Processing initial sync")

	// Track services that need to be processed after existing
	// allocations are recorded.
	var pendingServices []*kapi.Service

	// Track post-sync changes that need to be added back the queue
	// after allocations are recorded.  These changes may end up in
	// the queue if watch events were queued before completion of the
	// initial sync was detected.
	var pendingChanges []*serviceChange

	// Drain the queue.  Len() should be safe because enqueueService
	// requires the same lock held by this method.
	for ic.queue.Len() > 0 {
		item, quit := ic.queue.Get()
		if quit {
			return false
		}
		ic.queue.Done(item)
		ic.queue.Forget(item)
		change := item.(*serviceChange)

		// The initial sync only includes additions, so if an update
		// or delete is seen (indicated by the presence of oldService),
		// it and all subsequent changes are post-sync watch events that
		// should be queued without processing.
		postSyncChange := change.oldService != nil || len(pendingChanges) > 0
		if postSyncChange {
			pendingChanges = append(pendingChanges, change)
			continue
		}

		service := ic.getCachedService(change.key)
		if service == nil {
			// Service was deleted
			continue
		}

		if service.Spec.Type == kapi.ServiceTypeLoadBalancer {
			// Save for subsequent addition back to the queue to
			// ensure persistent state is updated during regular
			// processing.
			pendingServices = append(pendingServices, service)

			if len(service.Status.LoadBalancer.Ingress) > 0 {
				// The service has an existing allocation
				ipString := service.Status.LoadBalancer.Ingress[0].IP
				// Return values indicating that reallocation is
				// necessary or that an error occurred can be ignored
				// since the service will be processed again.
				ic.recordLocalAllocation(change.key, ipString)
			}
		}
	}

	// Add pending service additions back to the queue in consistent order.
	sort.Sort(serviceAge(pendingServices))
	for _, service := range pendingServices {
		if key, err := controller.KeyFunc(service); err == nil {
			glog.V(5).Infof("Adding service back to queue: %v ", key)
			change := &serviceChange{key: key}
			ic.queue.Add(change)
		} else {
			// This error should have been caught by enqueueService
			utilruntime.HandleError(fmt.Errorf("Couldn't get key for service %+v: %v", service, err))
			continue
		}
	}

	// Add watch events back to the queue
	for _, change := range pendingChanges {
		ic.queue.Add(change)
	}

	glog.V(5).Infof("Completed processing initial sync")

	return true
}

// getCachedService logs if unable to retrieve a service for the given key.
func (ic *IngressIPController) getCachedService(key string) *kapi.Service {
	if len(key) == 0 {
		return nil
	}
	if obj, exists, err := ic.cache.GetByKey(key); err != nil {
		glog.V(5).Infof("Unable to retrieve service %v from store: %v", key, err)
	} else if !exists {
		glog.V(6).Infof("Service %v has been deleted", key)
	} else {
		return obj.(*kapi.Service)
	}
	return nil
}

// recordLocalAllocation attempts to update local state for the given
// service key and ingress ip.  Returns a boolean indication of
// whether reallocation is necessary and an error indicating the
// reason for reallocation.  If reallocation is not indicated, a
// non-nil error indicates an exceptional condition.
func (ic *IngressIPController) recordLocalAllocation(key, ipString string) (reallocate bool, err error) {
	ip := net.ParseIP(ipString)
	if ip == nil {
		return true, fmt.Errorf("Service %v has an invalid ingress ip %v.  A new ip will be allocated.", key, ipString)
	}

	ipKey, ok := ic.allocationMap[ipString]
	switch {
	case ok && ipKey == key:
		// Allocation exists for this service
		return false, nil
	case ok && ipKey != key:
		// TODO prefer removing the allocation from a service that does not have a matching LoadBalancerIP
		return true, fmt.Errorf("Another service is using ingress ip %v.  A new ip will be allocated for %v.", ipString, key)
	}

	err = ic.ipAllocator.Allocate(ip)
	switch {
	case err == ipallocator.ErrNotInRange:
		return true, fmt.Errorf("The ingress ip %v for service %v is not in the ingress range.  A new ip will be allocated.", ipString, key)
	case err != nil:
		// The only other error that Allocate() can throw is ErrAllocated, but that
		// should not happen after the check against the allocation map.
		return false, fmt.Errorf("Unexpected error from ip allocator for service %v: %v", key, err)
	}
	ic.allocationMap[ipString] = key
	glog.V(5).Infof("Recorded allocation of ip %v for service %v", ipString, key)
	return false, nil
}

// work dispatches the next item in the queue to the change handler.
// If the change handler returns an error, the change will be added to
// the end of the queue to be processed again.  Returns a boolean
// indication of whether processing should continue.
func (ic *IngressIPController) work() bool {
	item, quit := ic.queue.Get()
	if quit {
		return false
	}
	change := item.(*serviceChange)
	defer ic.queue.Done(change)

	if change.requeuedAllocation {
		// Reset the allocation state so that the change can be
		// requeued if necessary.  Only additions/updates are requeued
		// for allocation so change.key should be non-empty.
		change.requeuedAllocation = false
		ic.requeuedAllocations.Delete(change.key)
	}

	if err := ic.changeHandler(change); err == nil {
		// No further processing required
		ic.queue.Forget(change)
	} else {
		if err == ipallocator.ErrFull {
			// When the range is full, avoid requeueing more than a
			// single change requiring allocation per service.
			// Otherwise the queue could grow without bounds as every
			// service update would add another change that would be
			// endlessly requeued.
			if ic.requeuedAllocations.Has(change.key) {
				return true
			}
			change.requeuedAllocation = true
			ic.requeuedAllocations.Insert(change.key)
			service := ic.getCachedService(change.key)
			if service != nil {
				ic.recorder.Eventf(service, kapi.EventTypeWarning, "IngressIPRangeFull", "No available ingress ip to allocate to service %s", change.key)
			}
		}
		// Failed but can be retried
		utilruntime.HandleError(fmt.Errorf("error syncing service, it will be retried: %v", err))
		ic.queue.AddRateLimited(change)
	}

	return true
}

// processChange responds to a service change by synchronizing the
// local and persisted ingress ip allocation state of the service.
func (ic *IngressIPController) processChange(change *serviceChange) error {
	service := ic.getCachedService(change.key)

	ic.clearOldAllocation(service, change.oldService)

	if service == nil {
		// Service was deleted - no further processing required
		return nil
	}

	typeLoadBalancer := service.Spec.Type == kapi.ServiceTypeLoadBalancer
	hasAllocation := len(service.Status.LoadBalancer.Ingress) > 0
	switch {
	case typeLoadBalancer && hasAllocation:
		return ic.recordAllocation(service, change.key)
	case typeLoadBalancer && !hasAllocation:
		return ic.allocate(service, change.key)
	case !typeLoadBalancer && hasAllocation:
		return ic.deallocate(service, change.key)
	default:
		return nil
	}
}

// clearOldAllocation clears the old allocation for a service if it
// differs from a new allocation.  Returns a boolean indication of
// whether the old allocation was cleared.
func (ic *IngressIPController) clearOldAllocation(new, old *kapi.Service) bool {
	oldIP := ""
	if old != nil && old.Spec.Type == kapi.ServiceTypeLoadBalancer && len(old.Status.LoadBalancer.Ingress) > 0 {
		oldIP = old.Status.LoadBalancer.Ingress[0].IP
	}
	noOldAllocation := len(oldIP) == 0
	if noOldAllocation {
		return false
	}

	newIP := ""
	if new != nil && new.Spec.Type == kapi.ServiceTypeLoadBalancer && len(new.Status.LoadBalancer.Ingress) > 0 {
		newIP = new.Status.LoadBalancer.Ingress[0].IP
	}
	allocationUnchanged := newIP == oldIP
	if allocationUnchanged {
		return false
	}

	// New allocation differs from old due to update or deletion

	// Get the key from the old service since the new service may be nil
	if key, err := controller.KeyFunc(old); err == nil {
		ic.clearLocalAllocation(key, oldIP)
		return true
	} else {
		// Recovery/retry not possible for this error
		utilruntime.HandleError(fmt.Errorf("Couldn't get key for object %+v: %v", old, err))
		return false
	}
}

// recordAllocation updates local state with the ingress ip indicated
// in a service's status and ensures that the ingress ip appears in
// the service's list of external ips.  If the service's ingress ip is
// invalid for any reason, a new ip will be allocated.
func (ic *IngressIPController) recordAllocation(service *kapi.Service, key string) error {
	// If more than one ingress ip is present, it will be ignored
	ipString := service.Status.LoadBalancer.Ingress[0].IP

	reallocate, err := ic.recordLocalAllocation(key, ipString)
	if !reallocate && err != nil {
		return err
	}
	reallocateMessage := ""
	if err != nil {
		reallocateMessage = err.Error()
	}

	// Make a copy to modify to avoid mutating cache state
	t, err := kapi.Scheme.DeepCopy(service)
	if err != nil {
		return err
	}
	service = t.(*kapi.Service)

	if reallocate {
		// TODO update the external ips but not the status since
		// allocate() will overwrite any existing allocation.
		if err = ic.clearPersistedAllocation(service, key, reallocateMessage); err != nil {
			return err
		}
		ic.recorder.Eventf(service, kapi.EventTypeWarning, "IngressIPReallocated", reallocateMessage)
		return ic.allocate(service, key)
	} else {
		// Ensure that the ingress ip is present in the service's spec.
		return ic.ensureExternalIP(service, key, ipString)
	}
}

// allocate assigns an unallocated ip to a service and updates the
// service's persisted state.
func (ic *IngressIPController) allocate(service *kapi.Service, key string) error {
	// Make a copy to avoid mutating cache state
	t, err := kapi.Scheme.DeepCopy(service)
	if err != nil {
		return err
	}
	service = t.(*kapi.Service)

	ip, err := ic.allocateIP(service.Spec.LoadBalancerIP)
	if err != nil {
		return err
	}
	ipString := ip.String()

	glog.V(5).Infof("Allocating ip %v to service %v", ipString, key)
	service.Status = kapi.ServiceStatus{
		LoadBalancer: kapi.LoadBalancerStatus{
			Ingress: []kapi.LoadBalancerIngress{
				{
					IP: ipString,
				},
			},
		},
	}
	if err = ic.persistServiceStatus(service); err != nil {
		if releaseErr := ic.ipAllocator.Release(ip); releaseErr != nil {
			// Release from contiguous allocator should never return an error, but just in case...
			utilruntime.HandleError(fmt.Errorf("Error releasing ip %v for service %v: %v", ipString, key, releaseErr))
		}
		return err
	}
	ic.allocationMap[ipString] = key

	return ic.ensureExternalIP(service, key, ipString)
}

// deallocate ensures that the ip currently allocated to a service is
// removed and that its loadbalancer status is cleared.
func (ic *IngressIPController) deallocate(service *kapi.Service, key string) error {
	glog.V(5).Infof("Clearing allocation state for %v", key)

	// Make a copy to modify to avoid mutating cache state
	t, err := kapi.Scheme.DeepCopy(service)
	if err != nil {
		return err
	}
	service = t.(*kapi.Service)

	// Get the ingress ip to remove from local allocation state before
	// it is removed from the service.
	ipString := service.Status.LoadBalancer.Ingress[0].IP

	if err = ic.clearPersistedAllocation(service, key, ""); err != nil {
		return err
	}

	ic.clearLocalAllocation(key, ipString)
	return nil
}

// clearLocalAllocation clears an in-memory allocation if it belongs
// to the specified service key.
func (ic *IngressIPController) clearLocalAllocation(key, ipString string) bool {
	glog.V(5).Infof("Attempting to clear local allocation of ip %v for service %v", ipString, key)

	ip := net.ParseIP(ipString)
	if ip == nil {
		// An invalid ip address cannot be deallocated
		utilruntime.HandleError(fmt.Errorf("Error parsing ip: %v", ipString))
		return false
	}

	ipKey, ok := ic.allocationMap[ipString]
	switch {
	case !ok:
		glog.V(6).Infof("IP address %v is not currently allocated", ipString)
		return false
	case key != ipKey:
		glog.V(6).Infof("IP address %v is not allocated to service %v", ipString, key)
		return false
	}

	// Remove allocation
	if err := ic.ipAllocator.Release(ip); err != nil {
		// Release from contiguous allocator should never return an error.
		utilruntime.HandleError(fmt.Errorf("Error releasing ip %v for service %v: %v", ipString, key, err))
		return false
	}
	delete(ic.allocationMap, ipString)
	glog.V(5).Infof("IP address %v is now available for allocation", ipString)
	return true
}

// clearPersistedAllocation ensures there is no ingress ip in the
// service's spec and that the service's status is cleared.
func (ic *IngressIPController) clearPersistedAllocation(service *kapi.Service, key, errMessage string) error {
	// Assume it is safe to modify the service without worrying about changing the local cache

	if len(errMessage) > 0 {
		utilruntime.HandleError(fmt.Errorf(errMessage))
	} else {
		glog.V(5).Infof("Attempting to clear persisted allocation for service: %v", key)
	}

	// An ingress ip is only allowed in ExternalIPs when a
	// corresponding status exists, so update the spec first to avoid
	// failing admission control.
	ingressIP := service.Status.LoadBalancer.Ingress[0].IP
	for i, ip := range service.Spec.ExternalIPs {
		if ip == ingressIP {
			glog.V(5).Infof("Removing ip %v from the external ips of service %v", ingressIP, key)
			service.Spec.ExternalIPs = append(service.Spec.ExternalIPs[:i], service.Spec.ExternalIPs[i+1:]...)
			if err := ic.persistServiceSpec(service); err != nil {
				return err
			}
			break
		}
	}

	service.Status.LoadBalancer = kapi.LoadBalancerStatus{}
	glog.V(5).Infof("Clearing the load balancer status of service: %v", key)
	return ic.persistServiceStatus(service)
}

// ensureExternalIP ensures that the provided service has the ingress
// ip persisted as an external ip.
func (ic *IngressIPController) ensureExternalIP(service *kapi.Service, key, ingressIP string) error {
	// Assume it is safe to modify the service without worrying about changing the local cache

	ipExists := false
	for _, ip := range service.Spec.ExternalIPs {
		if ip == ingressIP {
			ipExists = true
			glog.V(6).Infof("Service %v already has ip %v as an external ip", key, ingressIP)
			break
		}
	}
	if !ipExists {
		service.Spec.ExternalIPs = append(service.Spec.ExternalIPs, ingressIP)
		glog.V(5).Infof("Adding ip %v to service %v as an external ip", ingressIP, key)
		return ic.persistServiceSpec(service)
	}
	return nil
}

// allocateIP attempts to allocate the requested ip, and if that is
// not possible, allocates the next available address.
func (ic *IngressIPController) allocateIP(requestedIP string) (net.IP, error) {
	if len(requestedIP) == 0 {
		// Specific ip not requested
		return ic.ipAllocator.AllocateNext()
	}
	var ip net.IP
	if ip = net.ParseIP(requestedIP); ip == nil {
		// Invalid ip
		return ic.ipAllocator.AllocateNext()
	}
	if err := ic.ipAllocator.Allocate(ip); err != nil {
		// Unable to allocate requested ip
		return ic.ipAllocator.AllocateNext()
	}
	// Allocated requested ip
	return ip, nil
}

func (ic *IngressIPController) persistServiceSpec(service *kapi.Service) error {
	return ic.persistenceHandler(ic.client, service, false)
}

func (ic *IngressIPController) persistServiceStatus(service *kapi.Service) error {
	return ic.persistenceHandler(ic.client, service, true)
}

func persistService(client kclient.ServicesNamespacer, service *kapi.Service, targetStatus bool) error {
	backoff := wait.Backoff{
		Steps:    clientRetryCount,
		Duration: clientRetryInterval,
		Factor:   clientRetryFactor,
	}
	return wait.ExponentialBackoff(backoff, func() (bool, error) {
		var err error
		if targetStatus {
			_, err = client.Services(service.Namespace).UpdateStatus(service)
		} else {
			_, err = client.Services(service.Namespace).Update(service)
		}
		switch {
		case err == nil:
			return true, nil
		case kerrors.IsNotFound(err):
			// If the service no longer exists, we don't want to recreate
			// it. Just bail out so that we can process the delete, which
			// we should soon be receiving if we haven't already.
			glog.V(5).Infof("Not persisting update to service '%s/%s' that no longer exists: %v",
				service.Namespace, service.Name, err)
			return true, nil
		case kerrors.IsConflict(err):
			// TODO: Try to resolve the conflict if the change was
			// unrelated to load balancer status. For now, just rely on
			// the fact that we'll also process the update that caused the
			// resource version to change.
			glog.V(5).Infof("Not persisting update to service '%s/%s' that has been changed since we received it: %v",
				service.Namespace, service.Name, err)
			return true, nil
		default:
			err = fmt.Errorf("Failed to persist updated LoadBalancerStatus to service '%s/%s': %v",
				service.Namespace, service.Name, err)
			return false, err
		}
	})
}
