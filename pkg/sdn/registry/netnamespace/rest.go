package netnamespace

import (
	"fmt"

	"github.com/golang/glog"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/rest"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/registry/service"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/fielderrors"
	"k8s.io/kubernetes/pkg/util/sets"
	"k8s.io/kubernetes/pkg/watch"

	"github.com/openshift/origin/pkg/sdn/api"
	"github.com/openshift/origin/pkg/sdn/api/validation"
	"github.com/openshift/origin/pkg/sdn/registry/netnamespace/cache"
	"github.com/openshift/origin/pkg/sdn/registry/netnamespace/vnid"
	"github.com/openshift/origin/pkg/sdn/registry/netnamespace/vnidallocator"
	"github.com/openshift/origin/pkg/sdn/registry/netnamespace/vnidallocator/etcd"
)

// REST adapts a netnamespace registry into apiserver's RESTStorage model.
type REST struct {
	registry     Registry
	vnids        vnidallocator.Interface
	vnidRegistry service.RangeRegistry
	// globalNamespaces are the namespaces that have access to all pods in the cluster and vice versa.
	globalNamespaces sets.String
}

// NewStorage returns a new REST.
func NewStorage(registry Registry, vnids vnidallocator.Interface, vnidRegistry service.RangeRegistry, globalNamespaces []string) *REST {
	return &REST{
		registry:         registry,
		vnids:            vnids,
		vnidRegistry:     vnidRegistry,
		globalNamespaces: sets.NewString(globalNamespaces...),
	}
}

func (rs *REST) Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error) {
	netns := obj.(*api.NetNamespace)

	if err := rest.BeforeCreate(Strategy, ctx, obj); err != nil {
		return nil, err
	}

	err := rs.assignNetID(netns, true)
	if err != nil {
		return nil, err
	}

	out, err := rs.registry.CreateNetNamespace(ctx, netns)
	if err != nil {
		er := rs.revokeNetID(netns)
		if er != nil {
			// these should be caught by an eventual reconciliation / restart
			glog.Errorf("Error releasing netnamespace %s NetID %d: %v", netns.Name, netns.NetID, er)
		}
		er = rest.CheckGeneratedNameError(Strategy, err, netns)
		if er != nil {
			return nil, er
		}
	}

	return out, err
}

func (rs *REST) Delete(ctx kapi.Context, id string) (runtime.Object, error) {
	netns, err := rs.registry.GetNetNamespace(ctx, id)
	if err != nil {
		return nil, err
	}

	err = rs.registry.DeleteNetNamespace(ctx, id)
	if err != nil {
		return nil, err
	}

	err = rs.revokeNetID(netns)
	if err != nil {
		// these should be caught by an eventual reconciliation / restart
		glog.Errorf("Error releasing netnamespace %s NetID %d: %v", netns.Name, netns.NetID, err)
	}
	return &unversioned.Status{Status: unversioned.StatusSuccess}, nil
}

func (rs *REST) Get(ctx kapi.Context, id string) (runtime.Object, error) {
	return rs.registry.GetNetNamespace(ctx, id)
}

func (rs *REST) List(ctx kapi.Context, label labels.Selector, field fields.Selector) (runtime.Object, error) {
	return rs.registry.ListNetNamespaces(ctx, label, field)
}

// Watch returns NetNamespaces events via a watch.Interface.
// It implements rest.Watcher.
func (rs *REST) Watch(ctx kapi.Context, label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error) {
	return rs.registry.WatchNetNamespaces(ctx, label, field, resourceVersion)
}

func (*REST) New() runtime.Object {
	return &api.NetNamespace{}
}

func (*REST) NewList() runtime.Object {
	return &api.NetNamespaceList{}
}

func (rs *REST) Update(ctx kapi.Context, obj runtime.Object) (runtime.Object, bool, error) {
	netns := obj.(*api.NetNamespace)
	oldNetns, err := rs.registry.GetNetNamespace(ctx, netns.Name)
	if err != nil {
		return nil, false, err
	}

	if errs := validation.ValidateNetNamespaceUpdate(oldNetns, netns); len(errs) > 0 {
		return nil, false, errors.NewInvalid("netNamespace", netns.Name, errs)
	}

	createdNetID := false
	changedNetID := true
	if netns.NetID == nil {
		err = rs.assignNetID(netns, false)
		if err != nil {
			return nil, false, err
		}
		createdNetID = true
	} else if *oldNetns.NetID == *netns.NetID {
		changedNetID = false
	} else if *netns.NetID != vnid.GlobalVNID {
		err = rs.checkNetID(netns)
		if err != nil {
			return nil, false, err
		}
	}

	out, err := rs.registry.UpdateNetNamespace(ctx, netns)
	if err != nil {
		if createdNetID {
			er := rs.revokeNetID(netns)
			if er != nil {
				// these should be caught by an eventual reconciliation / restart
				glog.Errorf("Error releasing netnamespace %s NetID %d: %v", netns.Name, *netns.NetID, er)
			}
		}
		return nil, false, err
	}

	if changedNetID {
		er := rs.revokeNetID(oldNetns)
		if er != nil {
			// these should be caught by an eventual reconciliation / restart
			glog.Errorf("Error releasing netnamespace %s NetID %d: %v", oldNetns.Name, *oldNetns.NetID, er)
		}
	}
	return out, false, nil
}

func (rs *REST) assignNetID(netns *api.NetNamespace, checkGlobalNs bool) error {
	// checkGlobalNs is set to true in case of Create() and false in case of Update()
	if checkGlobalNs && rs.isGlobalNamespace(netns) {
		netns.NetID = new(uint)
		*netns.NetID = vnid.GlobalVNID
	} else if netns.NetID != nil {
		// Try to respect the requested Net ID.
		if err := rs.vnids.Allocate(*netns.NetID); err != nil {
			el := fielderrors.ValidationErrorList{fielderrors.NewFieldInvalid("NetID", netns.NetID, err.Error())}
			return errors.NewInvalid("NetNamespace", netns.Name, el)
		}
	} else {
		// Allocate next available.
		id, err := rs.vnids.AllocateNext()
		if err != nil {
			el := fielderrors.ValidationErrorList{fielderrors.NewFieldInvalid("NetID", netns.NetID, err.Error())}
			return errors.NewInvalid("NetNamespace", netns.Name, el)
		}
		netns.NetID = &id
	}
	return nil

}

func (rs *REST) revokeNetID(netns *api.NetNamespace) error {
	// Skip GlobalVNID as it is not part of Net ID allocation
	if *netns.NetID == vnid.GlobalVNID {
		return nil
	}

	netnsCache, err := cache.GetNetNamespaceCache()
	if err != nil {
		return err
	}

	// Retry the op if Release() returns ErrorRetryOperation
	for i := 0; i < 2; i++ {
		// Don't release if this netid is used by any other namespaces
		for _, obj := range netnsCache.Store.List() {
			nn := obj.(*api.NetNamespace)
			if nn.ObjectMeta.UID == netns.ObjectMeta.UID {
				continue
			}
			if *nn.NetID == *netns.NetID {
				return nil
			}
		}
		err = rs.vnids.Release(*netns.NetID)
		if err != etcd.ErrorRetryOperation {
			return err
		}
	}
	return err
}

func (rs *REST) checkNetID(netns *api.NetNamespace) error {
	if !rs.vnids.Has(*netns.NetID) {
		return fmt.Errorf("NetID %d is not allocated, you can only use existing NetID during update", *netns.NetID)
	}

	// Update vnid allocation resource
	// This will synchronize update and revoke vnid operations
	latest, err := rs.vnidRegistry.Get()
	if err != nil {
		return err
	}
	if err := rs.vnidRegistry.CreateOrUpdate(latest); err != nil {
		return fmt.Errorf("unable to persist the updated vnid allocations: %v", err)
	}
	return nil
}

// isGlobalNamespace returns true in these cases:
// - when NetID = vnid.GlobalVNID or
// - NetName is in the rs.globalNamespaces
func (rs *REST) isGlobalNamespace(netns *api.NetNamespace) bool {
	if (netns.NetID != nil) && (*netns.NetID == vnid.GlobalVNID) {
		return true
	}
	if rs.globalNamespaces.Has(netns.NetName) {
		return true
	}
	return false
}
