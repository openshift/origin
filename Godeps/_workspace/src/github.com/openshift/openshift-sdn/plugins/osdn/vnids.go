package osdn

import (
	"fmt"
	"strings"
	"time"

	log "github.com/golang/glog"

	kapi "k8s.io/kubernetes/pkg/api"
	kubetypes "k8s.io/kubernetes/pkg/kubelet/container"
	kerrors "k8s.io/kubernetes/pkg/util/errors"
	utilruntime "k8s.io/kubernetes/pkg/util/runtime"
	"k8s.io/kubernetes/pkg/util/sets"
	utilwait "k8s.io/kubernetes/pkg/util/wait"
	"k8s.io/kubernetes/pkg/watch"

	"github.com/openshift/openshift-sdn/pkg/netutils"
	osapi "github.com/openshift/origin/pkg/sdn/api"
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

	go utilwait.Forever(oc.watchNamespaces, 0)
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

func (oc *OsdnController) watchNamespaces() {
	eventQueue := oc.Registry.RunEventQueue(Namespaces)

	for {
		eventType, obj, err := eventQueue.Pop()
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("EventQueue failed for namespaces: %v", err))
			return
		}
		ns := obj.(*kapi.Namespace)
		name := ns.ObjectMeta.Name

		log.V(5).Infof("Watch %s event for Namespace %q", strings.Title(string(eventType)), name)
		switch eventType {
		case watch.Added, watch.Modified:
			err := oc.assignVNID(name)
			if err != nil {
				log.Errorf("Error assigning netid: %v", err)
				continue
			}
		case watch.Deleted:
			err := oc.revokeVNID(name)
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

	go utilwait.Forever(oc.watchNetNamespaces, 0)
	go utilwait.Forever(oc.watchServices, 0)
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
	errList := []error{}
	for _, svc := range services {
		if err := oc.pluginHooks.DeleteServiceRules(&svc); err != nil {
			log.Error(err)
		}
		if err := oc.pluginHooks.AddServiceRules(&svc, netID); err != nil {
			errList = append(errList, err)
		}
	}
	return kerrors.NewAggregate(errList)
}

func (oc *OsdnController) watchNetNamespaces() {
	eventQueue := oc.Registry.RunEventQueue(NetNamespaces)

	for {
		eventType, obj, err := eventQueue.Pop()
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("EventQueue failed for network namespaces: %v", err))
			return
		}
		netns := obj.(*osapi.NetNamespace)

		log.V(5).Infof("Watch %s event for NetNamespace %q", strings.Title(string(eventType)), netns.ObjectMeta.Name)
		switch eventType {
		case watch.Added, watch.Modified:
			// Skip this event if the old and new network ids are same
			oldNetID, err := oc.GetVNID(netns.NetName)
			if (err == nil) && (oldNetID == netns.NetID) {
				continue
			}
			oc.setVNID(netns.NetName, netns.NetID)

			err = oc.updatePodNetwork(netns.NetName, netns.NetID)
			if err != nil {
				log.Errorf("Failed to update pod network for namespace '%s', error: %s", netns.NetName, err)
				oc.setVNID(netns.NetName, oldNetID)
				continue
			}
		case watch.Deleted:
			// updatePodNetwork needs vnid, so unset vnid after this call
			err := oc.updatePodNetwork(netns.NetName, AdminVNID)
			if err != nil {
				log.Errorf("Failed to update pod network for namespace '%s', error: %s", netns.NetName, err)
			}
			oc.unSetVNID(netns.NetName)
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

func (oc *OsdnController) watchServices() {
	services := make(map[string]*kapi.Service)
	eventQueue := oc.Registry.RunEventQueue(Services)

	for {
		eventType, obj, err := eventQueue.Pop()
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("EventQueue failed for services: %v", err))
			return
		}
		serv := obj.(*kapi.Service)

		// Ignore headless services
		if !kapi.IsServiceIPSet(serv) {
			continue
		}

		log.V(5).Infof("Watch %s event for Service %q", strings.Title(string(eventType)), serv.ObjectMeta.Name)
		switch eventType {
		case watch.Added, watch.Modified:
			oldsvc, exists := services[string(serv.UID)]
			if exists {
				if !isServiceChanged(oldsvc, serv) {
					continue
				}
				if err := oc.pluginHooks.DeleteServiceRules(oldsvc); err != nil {
					log.Error(err)
				}
			}

			netid, err := oc.WaitAndGetVNID(serv.Namespace)
			if err != nil {
				log.Errorf("Skipped adding service rules for serviceEvent: %v, Error: %v", eventType, err)
				continue
			}

			if err := oc.pluginHooks.AddServiceRules(serv, netid); err != nil {
				log.Error(err)
				continue
			}
			services[string(serv.UID)] = serv
		case watch.Deleted:
			delete(services, string(serv.UID))

			if err := oc.pluginHooks.DeleteServiceRules(serv); err != nil {
				log.Error(err)
			}
		}
	}
}
