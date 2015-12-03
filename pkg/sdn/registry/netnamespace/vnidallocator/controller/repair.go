package controller

import (
	"fmt"
	"time"

	"github.com/golang/glog"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/registry/service"
	kutil "k8s.io/kubernetes/pkg/util"

	projectcache "github.com/openshift/origin/pkg/project/cache"
	sdnapi "github.com/openshift/origin/pkg/sdn/api"
	"github.com/openshift/origin/pkg/sdn/registry/netnamespace"
	netnscache "github.com/openshift/origin/pkg/sdn/registry/netnamespace/cache"
	"github.com/openshift/origin/pkg/sdn/registry/netnamespace/vnid"
	"github.com/openshift/origin/pkg/sdn/registry/netnamespace/vnidallocator"
)

var checkNetNsForProject bool

type Repair struct {
	interval  time.Duration
	registry  netnamespace.Registry
	vnidRange vnid.VNIDRange
	alloc     service.RangeRegistry
}

// NewRepair creates a controller that periodically ensures that VNIDs are allocated for all namespaces
// when using multitenant network plugin and generates informational warnings like VNID leaks, etc.
func NewRepair(interval time.Duration, registry netnamespace.Registry, vnidRange vnid.VNIDRange, alloc service.RangeRegistry) *Repair {
	return &Repair{
		interval:  interval,
		registry:  registry,
		vnidRange: vnidRange,
		alloc:     alloc,
	}
}

// RunUntil starts the controller until the provided ch is closed.
func (c *Repair) RunUntil(ch chan struct{}) {
	kutil.Until(func() {
		if err := c.RunOnce(); err != nil {
			kutil.HandleError(err)
		}
	}, c.interval, ch)
}

// RunOnce verifies the state of the vnid allocations and returns an error if an unrecoverable problem occurs.
func (c *Repair) RunOnce() error {
	// TODO: (per smarterclayton) if Get() or ListNetNamespaces() is a weak consistency read,
	// or if they are executed against different leaders,
	// the ordering guarantee required to ensure no vnid is allocated twice is violated.
	// ListNetNamespaces must return a ResourceVersion higher than the etcd index Get triggers,
	// and the release code must not release netnamespaces that have had vnids allocated but not yet been created
	// See #8295

	// If etcd server is not running we should wait for some time and fail only then. This is particularly
	// important when we start apiserver and etcd at the same time.
	var latest *kapi.RangeAllocation
	var err error
	for i := 0; i < 10; i++ {
		if latest, err = c.alloc.Get(); err != nil {
			time.Sleep(time.Second)
		} else {
			break
		}
	}
	if err != nil {
		return fmt.Errorf("unable to refresh the vnid block: %v", err)
	}

	netnsCache, err := netnscache.GetNetNamespaceCache()
	if err != nil {
		return err
	}

	netnsList := netnsCache.Store.List()
	netnsMap := make(map[string]bool, len(netnsList))
	netIDCountMap := make(map[uint]int, len(netnsList))
	for _, obj := range netnsList {
		netns := obj.(*sdnapi.NetNamespace)
		netnsMap[netns.NetName] = true
		netIDCountMap[*netns.NetID] += 1
	}

	// When the cluster admin switches from flat network plugin to multitenant plugin,
	// NetNamespace object won't be present for the corresponding project as this controller
	// is started before the multitenant VnidStartMaster().
	// To avoid false errors, skip the check for the first iteration
	if checkNetNsForProject {
		// Validate every project has a corresponding netnamespace object
		projectCache, err := projectcache.GetProjectCache()
		if err != nil {
			return fmt.Errorf("unable to get project cache: %v", err)
		}
		for _, p := range projectCache.Store.List() {
			namespace := p.(*kapi.Namespace)
			if !netnsMap[namespace.ObjectMeta.Name] {
				// There could be a race condition when the namesapce is created
				// and netnamespace is not yet created but this repair controller
				// tries to validate the netnamespace presence.
				// To avoid this issue, log error only if the namespace is created a few mins ago.
				// We expect NetNamespace resource to be created for the namespace
				// when we start/restart multitenant network plugin.
				if namespace.ObjectMeta.CreationTimestamp.Time.Add(5 * time.Minute).Before(time.Now()) {
					glog.Errorf("NetNamespace resource not found for namespace: %s", namespace.ObjectMeta.Name)
				}
			}
		}
	} else {
		checkNetNsForProject = true
	}

	r := vnidallocator.NewInMemoryAllocator(c.vnidRange)
	for _, obj := range netnsList {
		netns := obj.(*sdnapi.NetNamespace)

		// Skip GlobalVNID as it is not part of the VNID allocation
		if *netns.NetID == vnid.GlobalVNID {
			continue
		}
		switch err := r.Allocate(*netns.NetID); err {
		case nil:
			// Expected value
		case vnidallocator.ErrAllocated:
			// TODO: send event
			if netIDCountMap[*netns.NetID] == 1 {
				kutil.HandleError(fmt.Errorf("unexpected vnid %d allocated error for netnamespace %s", *netns.NetID, netns.NetName))
			}
		case vnidallocator.ErrNotInRange:
			// TODO: send event
			// vnid is broken, reallocate
			kutil.HandleError(fmt.Errorf("the vnid %d for netnamespace %s is not within the vnid range %v; please recreate", *netns.NetID, netns.NetName, c.vnidRange))
		case vnidallocator.ErrFull:
			// TODO: send event
			return fmt.Errorf("the vnid range %v is full; you must widen the vnid range in order to create new netnamespaces", c.vnidRange)
		default:
			return fmt.Errorf("unable to allocate vnid %d for netnamespace %s due to an unknown error: %v", *netns.NetID, netns.NetName, err)
		}
	}

	err = r.Snapshot(latest)
	if err != nil {
		return fmt.Errorf("unable to take snapshot of vnid allocations: %v", err)
	}

	if err := c.alloc.CreateOrUpdate(latest); err != nil {
		return fmt.Errorf("unable to persist the updated vnid allocations: %v", err)
	}
	return nil
}
