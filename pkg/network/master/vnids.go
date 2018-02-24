package master

import (
	"fmt"
	"sync"

	"github.com/golang/glog"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/watch"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	"github.com/openshift/origin/pkg/network"
	networkapi "github.com/openshift/origin/pkg/network/apis/network"
	"github.com/openshift/origin/pkg/network/common"
	networkclient "github.com/openshift/origin/pkg/network/generated/internalclientset"
	pnetid "github.com/openshift/origin/pkg/network/master/netid"
)

type masterVNIDMap struct {
	// Synchronizes assign, revoke and update VNID
	lock         sync.Mutex
	ids          map[string]uint32
	netIDManager *pnetid.Allocator

	adminNamespaces  sets.String
	allowRenumbering bool
}

func newMasterVNIDMap(allowRenumbering bool) *masterVNIDMap {
	netIDRange, err := pnetid.NewNetIDRange(network.MinVNID, network.MaxVNID)
	if err != nil {
		panic(err)
	}

	return &masterVNIDMap{
		netIDManager:     pnetid.NewInMemory(netIDRange),
		adminNamespaces:  sets.NewString(metav1.NamespaceDefault),
		ids:              make(map[string]uint32),
		allowRenumbering: allowRenumbering,
	}
}

func (vmap *masterVNIDMap) getVNID(name string) (uint32, bool) {
	id, found := vmap.ids[name]
	return id, found
}

func (vmap *masterVNIDMap) setVNID(name string, id uint32) {
	vmap.ids[name] = id
}

func (vmap *masterVNIDMap) unsetVNID(name string) (uint32, bool) {
	id, found := vmap.ids[name]
	delete(vmap.ids, name)
	return id, found
}

func (vmap *masterVNIDMap) getVNIDCount(id uint32) int {
	count := 0
	for _, netid := range vmap.ids {
		if id == netid {
			count = count + 1
		}
	}
	return count
}

func (vmap *masterVNIDMap) isAdminNamespace(nsName string) bool {
	if vmap.adminNamespaces.Has(nsName) {
		return true
	}
	return false
}

func (vmap *masterVNIDMap) populateVNIDs(networkClient networkclient.Interface) error {
	netnsList, err := networkClient.Network().NetNamespaces().List(metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, netns := range netnsList.Items {
		vmap.setVNID(netns.NetName, netns.NetID)

		// Skip GlobalVNID, not part of netID allocation range
		if netns.NetID == network.GlobalVNID {
			continue
		}

		switch err := vmap.netIDManager.Allocate(netns.NetID); err {
		case nil: // Expected normal case
		case pnetid.ErrAllocated: // Expected when project networks are joined
		default:
			return fmt.Errorf("unable to allocate netid %d: %v", netns.NetID, err)
		}
	}
	return nil
}

func (vmap *masterVNIDMap) allocateNetID(nsName string) (uint32, bool, error) {
	// Nothing to do if the netid is in the vnid map
	exists := false
	if netid, found := vmap.getVNID(nsName); found {
		exists = true
		return netid, exists, nil
	}

	// NetNamespace not found, so allocate new NetID
	var netid uint32
	if vmap.isAdminNamespace(nsName) {
		netid = network.GlobalVNID
	} else {
		var err error
		netid, err = vmap.netIDManager.AllocateNext()
		if err != nil {
			return 0, exists, err
		}
	}

	vmap.setVNID(nsName, netid)
	glog.Infof("Allocated netid %d for namespace %q", netid, nsName)
	return netid, exists, nil
}

func (vmap *masterVNIDMap) releaseNetID(nsName string) error {
	// Remove NetID from vnid map
	netid, found := vmap.unsetVNID(nsName)
	if !found {
		return fmt.Errorf("netid not found for namespace %q", nsName)
	}

	// Skip network.GlobalVNID as it is not part of NetID allocation
	if netid == network.GlobalVNID {
		return nil
	}

	// Check if this netid is used by any other namespaces
	// If not, then release the netid
	if count := vmap.getVNIDCount(netid); count == 0 {
		if err := vmap.netIDManager.Release(netid); err != nil {
			return fmt.Errorf("error while releasing netid %d for namespace %q, %v", netid, nsName, err)
		}
		glog.Infof("Released netid %d for namespace %q", netid, nsName)
	} else {
		glog.V(5).Infof("netid %d for namespace %q is still in use", netid, nsName)
	}
	return nil
}

func (vmap *masterVNIDMap) updateNetID(nsName string, action network.PodNetworkAction, args string) (uint32, error) {
	var netid uint32
	allocated := false

	// Check if the given namespace exists or not
	oldnetid, found := vmap.getVNID(nsName)
	if !found {
		return 0, fmt.Errorf("netid not found for namespace %q", nsName)
	}

	// Determine new network ID
	switch action {
	case network.GlobalPodNetwork:
		netid = network.GlobalVNID
	case network.JoinPodNetwork:
		joinNsName := args
		var found bool
		if netid, found = vmap.getVNID(joinNsName); !found {
			return 0, fmt.Errorf("netid not found for namespace %q", joinNsName)
		}
	case network.IsolatePodNetwork:
		if nsName == kapi.NamespaceDefault {
			return 0, fmt.Errorf("network isolation for namespace %q is not allowed", nsName)
		}
		// Check if the given namespace is already isolated
		if count := vmap.getVNIDCount(oldnetid); count == 1 {
			return oldnetid, nil
		}

		var err error
		netid, err = vmap.netIDManager.AllocateNext()
		if err != nil {
			return 0, err
		}
		allocated = true
	default:
		return 0, fmt.Errorf("invalid pod network action: %v", action)
	}

	// Release old network ID
	if err := vmap.releaseNetID(nsName); err != nil {
		if allocated {
			vmap.netIDManager.Release(netid)
		}
		return 0, err
	}

	// Set new network ID
	vmap.setVNID(nsName, netid)
	glog.Infof("Updated netid %d for namespace %q", netid, nsName)
	return netid, nil
}

// assignVNID, revokeVNID and updateVNID methods updates in-memory structs and persists etcd objects
func (vmap *masterVNIDMap) assignVNID(networkClient networkclient.Interface, nsName string) error {
	vmap.lock.Lock()
	defer vmap.lock.Unlock()

	netid, exists, err := vmap.allocateNetID(nsName)
	if err != nil {
		return err
	}

	if !exists {
		// Create NetNamespace Object and update vnid map
		netns := &networkapi.NetNamespace{
			TypeMeta:   metav1.TypeMeta{Kind: "NetNamespace"},
			ObjectMeta: metav1.ObjectMeta{Name: nsName},
			NetName:    nsName,
			NetID:      netid,
		}
		_, err := networkClient.Network().NetNamespaces().Create(netns)
		if err != nil {
			vmap.releaseNetID(nsName)
			return err
		}
	}
	return nil
}

func (vmap *masterVNIDMap) revokeVNID(networkClient networkclient.Interface, nsName string) error {
	vmap.lock.Lock()
	defer vmap.lock.Unlock()

	// Delete NetNamespace object
	if err := networkClient.Network().NetNamespaces().Delete(nsName, &metav1.DeleteOptions{}); err != nil {
		return err
	}

	if err := vmap.releaseNetID(nsName); err != nil {
		return err
	}
	return nil
}

func (vmap *masterVNIDMap) updateVNID(networkClient networkclient.Interface, origNetns *networkapi.NetNamespace) error {
	// Informer cache should not be mutated, so get a copy of the object
	netns := origNetns.DeepCopy()

	action, args, err := network.GetChangePodNetworkAnnotation(netns)
	if err == network.ErrorPodNetworkAnnotationNotFound {
		// Nothing to update
		return nil
	} else if !vmap.allowRenumbering {
		network.DeleteChangePodNetworkAnnotation(netns)
		_, _ = networkClient.Network().NetNamespaces().Update(netns)
		return fmt.Errorf("network plugin does not allow NetNamespace renumbering")
	}

	vmap.lock.Lock()
	defer vmap.lock.Unlock()

	netid, err := vmap.updateNetID(netns.NetName, action, args)
	if err != nil {
		return err
	}
	netns.NetID = netid
	network.DeleteChangePodNetworkAnnotation(netns)

	if _, err := networkClient.Network().NetNamespaces().Update(netns); err != nil {
		return err
	}
	return nil
}

//--------------------- Master methods ----------------------

func (master *OsdnMaster) VnidStartMaster() error {
	err := master.vnids.populateVNIDs(master.networkClient)
	if err != nil {
		return err
	}

	master.watchNamespaces()
	master.watchNetNamespaces()
	return nil
}

func (master *OsdnMaster) watchNamespaces() {
	funcs := common.InformerFuncs(&kapi.Namespace{}, master.handleAddOrUpdateNamespace, master.handleDeleteNamespace)
	master.kubeInformers.Core().InternalVersion().Namespaces().Informer().AddEventHandler(funcs)
}

func (master *OsdnMaster) handleAddOrUpdateNamespace(obj, _ interface{}, eventType watch.EventType) {
	ns := obj.(*kapi.Namespace)
	glog.V(5).Infof("Watch %s event for Namespace %q", eventType, ns.Name)
	if err := master.vnids.assignVNID(master.networkClient, ns.Name); err != nil {
		utilruntime.HandleError(fmt.Errorf("Error assigning netid: %v", err))
	}
}

func (master *OsdnMaster) handleDeleteNamespace(obj interface{}) {
	ns := obj.(*kapi.Namespace)
	glog.V(5).Infof("Watch %s event for Namespace %q", watch.Deleted, ns.Name)
	if err := master.vnids.revokeVNID(master.networkClient, ns.Name); err != nil {
		utilruntime.HandleError(fmt.Errorf("Error revoking netid: %v", err))
	}
}

func (master *OsdnMaster) watchNetNamespaces() {
	funcs := common.InformerFuncs(&networkapi.NetNamespace{}, master.handleAddOrUpdateNetNamespace, nil)
	master.networkInformers.Network().InternalVersion().NetNamespaces().Informer().AddEventHandler(funcs)
}

func (master *OsdnMaster) handleAddOrUpdateNetNamespace(obj, _ interface{}, eventType watch.EventType) {
	netns := obj.(*networkapi.NetNamespace)
	glog.V(5).Infof("Watch %s event for NetNamespace %q", eventType, netns.Name)

	err := master.vnids.updateVNID(master.networkClient, netns)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Error updating netid: %v", err))
	}
}
