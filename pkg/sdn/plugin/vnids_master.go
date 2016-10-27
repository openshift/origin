package plugin

import (
	"fmt"
	"sync"

	log "github.com/golang/glog"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/util/sets"
	utilwait "k8s.io/kubernetes/pkg/util/wait"

	osclient "github.com/openshift/origin/pkg/client"
	osapi "github.com/openshift/origin/pkg/sdn/api"
	pnetid "github.com/openshift/origin/pkg/sdn/plugin/netid"
)

type masterVNIDMap struct {
	// Synchronizes assign, revoke and update VNID
	lock         sync.Mutex
	ids          map[string]uint32
	netIDManager *pnetid.Allocator

	adminNamespaces sets.String
}

func newMasterVNIDMap() *masterVNIDMap {
	netIDRange, err := pnetid.NewNetIDRange(osapi.MinVNID, osapi.MaxVNID)
	if err != nil {
		panic(err)
	}

	return &masterVNIDMap{
		netIDManager:    pnetid.NewInMemory(netIDRange),
		adminNamespaces: sets.NewString(kapi.NamespaceDefault),
		ids:             make(map[string]uint32),
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

func (vmap *masterVNIDMap) populateVNIDs(osClient *osclient.Client) error {
	netnsList, err := osClient.NetNamespaces().List(kapi.ListOptions{})
	if err != nil {
		return err
	}

	for _, netns := range netnsList.Items {
		vmap.setVNID(netns.NetName, netns.NetID)

		// Skip GlobalVNID, not part of netID allocation range
		if netns.NetID == osapi.GlobalVNID {
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
		netid = osapi.GlobalVNID
	} else {
		var err error
		netid, err = vmap.netIDManager.AllocateNext()
		if err != nil {
			return 0, exists, err
		}
	}

	vmap.setVNID(nsName, netid)
	log.Infof("Allocated netid %d for namespace %q", netid, nsName)
	return netid, exists, nil
}

func (vmap *masterVNIDMap) releaseNetID(nsName string) error {
	// Remove NetID from vnid map
	netid, found := vmap.unsetVNID(nsName)
	if !found {
		return fmt.Errorf("netid not found for namespace %q", nsName)
	}

	// Skip osapi.GlobalVNID as it is not part of NetID allocation
	if netid == osapi.GlobalVNID {
		return nil
	}

	// Check if this netid is used by any other namespaces
	// If not, then release the netid
	if count := vmap.getVNIDCount(netid); count == 0 {
		if err := vmap.netIDManager.Release(netid); err != nil {
			return fmt.Errorf("Error while releasing netid %d for namespace %q, %v", netid, nsName, err)
		}
		log.Infof("Released netid %d for namespace %q", netid, nsName)
	} else {
		log.V(5).Infof("netid %d for namespace %q is still in use", netid, nsName)
	}
	return nil
}

func (vmap *masterVNIDMap) updateNetID(nsName string, action osapi.PodNetworkAction, args string) (uint32, error) {
	var netid uint32
	allocated := false

	// Check if the given namespace exists or not
	oldnetid, found := vmap.getVNID(nsName)
	if !found {
		return 0, fmt.Errorf("netid not found for namespace %q", nsName)
	}

	// Determine new network ID
	switch action {
	case osapi.GlobalPodNetwork:
		netid = osapi.GlobalVNID
	case osapi.JoinPodNetwork:
		joinNsName := args
		var found bool
		if netid, found = vmap.getVNID(joinNsName); !found {
			return 0, fmt.Errorf("netid not found for namespace %q", joinNsName)
		}
	case osapi.IsolatePodNetwork:
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
	log.Infof("Updated netid %d for namespace %q", netid, nsName)
	return netid, nil
}

// assignVNID, revokeVNID and updateVNID methods updates in-memory structs and persists etcd objects
func (vmap *masterVNIDMap) assignVNID(osClient *osclient.Client, nsName string) error {
	vmap.lock.Lock()
	defer vmap.lock.Unlock()

	netid, exists, err := vmap.allocateNetID(nsName)
	if err != nil {
		return err
	}

	if !exists {
		// Create NetNamespace Object and update vnid map
		netns := &osapi.NetNamespace{
			TypeMeta:   unversioned.TypeMeta{Kind: "NetNamespace"},
			ObjectMeta: kapi.ObjectMeta{Name: nsName},
			NetName:    nsName,
			NetID:      netid,
		}
		_, err := osClient.NetNamespaces().Create(netns)
		if err != nil {
			vmap.releaseNetID(nsName)
			return err
		}
	}
	return nil
}

func (vmap *masterVNIDMap) revokeVNID(osClient *osclient.Client, nsName string) error {
	vmap.lock.Lock()
	defer vmap.lock.Unlock()

	// Delete NetNamespace object
	if err := osClient.NetNamespaces().Delete(nsName); err != nil {
		return err
	}

	if err := vmap.releaseNetID(nsName); err != nil {
		return err
	}
	return nil
}

func (vmap *masterVNIDMap) updateVNID(osClient *osclient.Client, netns *osapi.NetNamespace) error {
	action, args, err := osapi.GetChangePodNetworkAnnotation(netns)
	if err == osapi.ErrorPodNetworkAnnotationNotFound {
		// Nothing to update
		return nil
	}

	vmap.lock.Lock()
	defer vmap.lock.Unlock()

	netid, err := vmap.updateNetID(netns.NetName, action, args)
	if err != nil {
		return err
	}
	netns.NetID = netid
	osapi.DeleteChangePodNetworkAnnotation(netns)

	if _, err := osClient.NetNamespaces().Update(netns); err != nil {
		return err
	}
	return nil
}

//--------------------- Master methods ----------------------

func (master *OsdnMaster) VnidStartMaster() error {
	err := master.vnids.populateVNIDs(master.osClient)
	if err != nil {
		return err
	}

	go utilwait.Forever(master.watchNamespaces, 0)
	go utilwait.Forever(master.watchNetNamespaces, 0)
	return nil
}

func (master *OsdnMaster) watchNamespaces() {
	RunEventQueue(master.kClient, Namespaces, func(delta cache.Delta) error {
		ns := delta.Object.(*kapi.Namespace)
		name := ns.ObjectMeta.Name

		log.V(5).Infof("Watch %s event for Namespace %q", delta.Type, name)
		switch delta.Type {
		case cache.Sync, cache.Added, cache.Updated:
			if err := master.vnids.assignVNID(master.osClient, name); err != nil {
				return fmt.Errorf("Error assigning netid: %v", err)
			}
		case cache.Deleted:
			if err := master.vnids.revokeVNID(master.osClient, name); err != nil {
				return fmt.Errorf("Error revoking netid: %v", err)
			}
		}
		return nil
	})
}

func (master *OsdnMaster) watchNetNamespaces() {
	RunEventQueue(master.osClient, NetNamespaces, func(delta cache.Delta) error {
		netns := delta.Object.(*osapi.NetNamespace)
		name := netns.ObjectMeta.Name

		log.V(5).Infof("Watch %s event for NetNamespace %q", delta.Type, name)
		switch delta.Type {
		case cache.Sync, cache.Added, cache.Updated:
			err := master.vnids.updateVNID(master.osClient, netns)
			if err != nil {
				return fmt.Errorf("Error updating netid: %v", err)
			}
		}
		return nil
	})
}
