package osdn

import (
	"fmt"
	"time"

	log "github.com/golang/glog"

	"github.com/openshift/openshift-sdn/pkg/netutils"

	kapi "k8s.io/kubernetes/pkg/api"
	kubetypes "k8s.io/kubernetes/pkg/kubelet/container"
	"k8s.io/kubernetes/pkg/util/sets"
)

const (
	// Maximum VXLAN Network Identifier as per RFC#7348
	MaxVNID = ((1 << 24) - 1)
	// VNID for the admin namespaces
	AdminVNID = uint(0)
)

func (oc *OsdnController) GetVNID(name string) (uint, error) {
	oc.vnidLock.Lock()
	defer oc.vnidLock.Unlock()

	if id, ok := oc.vnidMap[name]; ok {
		return id, nil
	}
	// In case of error, return some value which is not a valid VNID
	return MaxVNID + 1, fmt.Errorf("Failed to find netid for namespace: %s in vnid map", name)
}

// Nodes asynchronously watch for both NetNamespaces and services
// NetNamespaces populates vnid map and services/pod-setup depend on vnid map
// If for some reason, vnid map propagation from master to node is slow
// and if service/pod-setup tries to lookup vnid map then it may fail.
// So, use this method to alleviate this problem. This method will
// retry vnid lookup before giving up.
func (oc *OsdnController) WaitAndGetVNID(name string) (uint, error) {
	// Try few times up to 2 seconds
	retries := 20
	retryInterval := 100 * time.Millisecond
	for i := 0; i < retries; i++ {
		if id, err := oc.GetVNID(name); err == nil {
			return id, nil
		}
		time.Sleep(retryInterval)
	}

	// In case of error, return some value which is not a valid VNID
	return MaxVNID + 1, fmt.Errorf("Failed to find netid for namespace: %s in vnid map", name)
}

func (oc *OsdnController) setVNID(name string, id uint) {
	oc.vnidLock.Lock()
	defer oc.vnidLock.Unlock()

	oc.vnidMap[name] = id
	log.Infof("Associate netid %d to namespace %q", id, name)
}

func (oc *OsdnController) unSetVNID(name string) (id uint, err error) {
	oc.vnidLock.Lock()
	defer oc.vnidLock.Unlock()

	id, found := oc.vnidMap[name]
	if !found {
		// In case of error, return some value which is not a valid VNID
		return MaxVNID + 1, fmt.Errorf("Failed to find netid for namespace: %s in vnid map", name)
	}
	delete(oc.vnidMap, name)
	log.Infof("Dissociate netid %d from namespace %q", id, name)
	return id, nil
}

func (oc *OsdnController) checkVNID(id uint) bool {
	oc.vnidLock.Lock()
	defer oc.vnidLock.Unlock()

	for _, netid := range oc.vnidMap {
		if netid == id {
			return true
		}
	}
	return false
}

func (oc *OsdnController) getAllocatedVNIDs() []uint {
	oc.vnidLock.Lock()
	defer oc.vnidLock.Unlock()

	ids := []uint{}
	idSet := sets.Int{}
	for _, id := range oc.vnidMap {
		if id != AdminVNID {
			if !idSet.Has(int(id)) {
				ids = append(ids, id)
				idSet.Insert(int(id))
			}
		}
	}
	return ids
}

func populateVNIDMap(oc *OsdnController) error {
	nets, err := oc.Registry.GetNetNamespaces()
	if err != nil {
		return err
	}

	for _, net := range nets {
		oc.setVNID(net.Name, net.NetID)
	}
	return nil
}

func (oc *OsdnController) VnidStartMaster() error {
	err := populateVNIDMap(oc)
	if err != nil {
		return err
	}

	// VNID: 0 reserved for default namespace and can reach any network in the cluster
	// VNID: 1 to 9 are internally reserved for any special cases in the future
	oc.netIDManager, err = netutils.NewNetIDAllocator(10, MaxVNID, oc.getAllocatedVNIDs())
	if err != nil {
		return err
	}

	// 'default' namespace is currently always an admin namespace
	oc.adminNamespaces = append(oc.adminNamespaces, "default")

	go watchNamespaces(oc)
	return nil
}

func (oc *OsdnController) isAdminNamespace(nsName string) bool {
	for _, name := range oc.adminNamespaces {
		if name == nsName {
			return true
		}
	}
	return false
}

func (oc *OsdnController) assignVNID(namespaceName string) error {
	// Nothing to do if the netid is in the vnid map
	if _, err := oc.GetVNID(namespaceName); err == nil {
		return nil
	}

	// If NetNamespace is present, update vnid map
	netns, err := oc.Registry.GetNetNamespace(namespaceName)
	if err == nil {
		oc.setVNID(namespaceName, netns.NetID)
		return nil
	}

	// NetNamespace not found, so allocate new NetID
	var netid uint
	if oc.isAdminNamespace(namespaceName) {
		netid = AdminVNID
	} else {
		var err error
		netid, err = oc.netIDManager.GetNetID()
		if err != nil {
			return err
		}
	}

	// Create NetNamespace Object and update vnid map
	err = oc.Registry.WriteNetNamespace(namespaceName, netid)
	if err != nil {
		e := oc.netIDManager.ReleaseNetID(netid)
		if e != nil {
			log.Errorf("Error while releasing netid: %v", e)
		}
		return err
	}
	oc.setVNID(namespaceName, netid)
	return nil
}

func (oc *OsdnController) revokeVNID(namespaceName string) error {
	// Remove NetID from vnid map
	netid_found := true
	netid, err := oc.unSetVNID(namespaceName)
	if err != nil {
		log.Error(err)
		netid_found = false
	}

	// Delete NetNamespace object
	err = oc.Registry.DeleteNetNamespace(namespaceName)
	if err != nil {
		return err
	}

	// Skip NetID release if
	// - Value matches AdminVNID as it is not part of NetID allocation or
	// - NetID is not found in the vnid map
	if (netid == AdminVNID) || !netid_found {
		return nil
	}

	// Check if this netid is used by any other namespaces
	// If not, then release the netid
	if !oc.checkVNID(netid) {
		err = oc.netIDManager.ReleaseNetID(netid)
		if err != nil {
			return fmt.Errorf("Error while releasing netid %d for namespace %q, %v", netid, namespaceName, err)
		}
		log.Infof("Released netid %d for namespace %q", netid, namespaceName)
	} else {
		log.V(5).Infof("netid %d for namespace %q is still in use", netid, namespaceName)
	}
	return nil
}

func watchNamespaces(oc *OsdnController) {
	nsevent := make(chan *NamespaceEvent)
	go oc.Registry.WatchNamespaces(nsevent)
	for {
		ev := <-nsevent
		switch ev.Type {
		case Added:
			err := oc.assignVNID(ev.Namespace.Name)
			if err != nil {
				log.Errorf("Error assigning netid: %v", err)
				continue
			}
		case Deleted:
			err := oc.revokeVNID(ev.Namespace.Name)
			if err != nil {
				log.Errorf("Error revoking netid: %v", err)
				continue
			}
		}
	}
}

func (oc *OsdnController) VnidStartNode() error {
	// Populate vnid map synchronously so that existing services can fetch vnid
	err := populateVNIDMap(oc)
	if err != nil {
		return err
	}

	// Populate pod info map synchronously so that kube proxy can filter endpoints to support isolation
	err = oc.Registry.PopulatePodsByIP()
	if err != nil {
		return err
	}

	go watchNetNamespaces(oc)
	go watchPods(oc)
	go watchServices(oc)

	return nil
}

func (oc *OsdnController) updatePodNetwork(namespace string, netID uint) error {
	// Update OF rules for the existing/old pods in the namespace
	pods, err := oc.GetLocalPods(namespace)
	if err != nil {
		return err
	}
	for _, pod := range pods {
		err := oc.pluginHooks.UpdatePod(pod.Namespace, pod.Name, kubetypes.DockerID(GetPodContainerID(&pod)))
		if err != nil {
			return err
		}
	}

	// Update OF rules for the old services in the namespace
	services, err := oc.Registry.GetServicesForNamespace(namespace)
	if err != nil {
		return err
	}
	for _, svc := range services {
		oc.pluginHooks.DeleteServiceRules(&svc)
		oc.pluginHooks.AddServiceRules(&svc, netID)
	}
	return nil
}

func watchNetNamespaces(oc *OsdnController) {
	netNsEvent := make(chan *NetNamespaceEvent)
	go oc.Registry.WatchNetNamespaces(netNsEvent)
	for {
		ev := <-netNsEvent
		switch ev.Type {
		case Added:
			// Skip this event if the old and new network ids are same
			oldNetID, err := oc.GetVNID(ev.NetNamespace.NetName)
			if (err == nil) && (oldNetID == ev.NetNamespace.NetID) {
				continue
			}
			oc.setVNID(ev.NetNamespace.Name, ev.NetNamespace.NetID)

			err = oc.updatePodNetwork(ev.NetNamespace.NetName, ev.NetNamespace.NetID)
			if err != nil {
				log.Errorf("Failed to update pod network for namespace '%s', error: %s", ev.NetNamespace.NetName, err)
			}
		case Deleted:
			err := oc.updatePodNetwork(ev.NetNamespace.NetName, AdminVNID)
			if err != nil {
				log.Errorf("Failed to update pod network for namespace '%s', error: %s", ev.NetNamespace.NetName, err)
			}
			oc.unSetVNID(ev.NetNamespace.NetName)
		}
	}
}

func isServiceChanged(oldsvc, newsvc *kapi.Service) bool {
	if len(oldsvc.Spec.Ports) == len(newsvc.Spec.Ports) {
		for i := range oldsvc.Spec.Ports {
			if oldsvc.Spec.Ports[i].Protocol != newsvc.Spec.Ports[i].Protocol ||
				oldsvc.Spec.Ports[i].Port != newsvc.Spec.Ports[i].Port {
				return true
			}
		}
		return false
	}
	return true
}

func watchServices(oc *OsdnController) {
	svcevent := make(chan *ServiceEvent)
	services := make(map[string]*kapi.Service)
	go oc.Registry.WatchServices(svcevent)

	for {
		ev := <-svcevent
		switch ev.Type {
		case Added:
			netid, err := oc.WaitAndGetVNID(ev.Service.Namespace)
			if err != nil {
				log.Errorf("Skipped serviceEvent: %v, Error: %v", ev, err)
				continue
			}

			oldsvc, exists := services[string(ev.Service.UID)]
			if exists {
				if !isServiceChanged(oldsvc, ev.Service) {
					continue
				}
				oc.pluginHooks.DeleteServiceRules(oldsvc)
			}
			services[string(ev.Service.UID)] = ev.Service
			oc.pluginHooks.AddServiceRules(ev.Service, netid)
		case Deleted:
			delete(services, string(ev.Service.UID))
			oc.pluginHooks.DeleteServiceRules(ev.Service)
		}
	}
}

func watchPods(oc *OsdnController) {
	oc.Registry.WatchPods()
}
