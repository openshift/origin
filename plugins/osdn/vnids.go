package osdn

import (
	"fmt"

	log "github.com/golang/glog"

	"github.com/openshift/openshift-sdn/pkg/netutils"

	kapi "k8s.io/kubernetes/pkg/api"
	kubetypes "k8s.io/kubernetes/pkg/kubelet/container"
)

const (
	// Maximum VXLAN Network Identifier as per RFC#7348
	MaxVNID = ((1 << 24) - 1)
	// VNID for the admin namespaces
	AdminVNID = uint(0)
)

func (oc *OsdnController) VnidStartMaster() error {
	nets, _, err := oc.Registry.GetNetNamespaces()
	if err != nil {
		return err
	}
	inUse := make([]uint, 0)
	for _, net := range nets {
		if net.NetID != AdminVNID {
			inUse = append(inUse, net.NetID)
		}
		oc.VNIDMap[net.Name] = net.NetID
	}
	// VNID: 0 reserved for default namespace and can reach any network in the cluster
	// VNID: 1 to 9 are internally reserved for any special cases in the future
	oc.netIDManager, err = netutils.NewNetIDAllocator(10, MaxVNID, inUse)
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
	_, err := oc.Registry.GetNetNamespace(namespaceName)
	if err == nil {
		return nil
	}
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
	err = oc.Registry.WriteNetNamespace(namespaceName, netid)
	if err != nil {
		e := oc.netIDManager.ReleaseNetID(netid)
		if e != nil {
			log.Errorf("Error while releasing Net ID: %v", e)
		}
		return err
	}
	oc.VNIDMap[namespaceName] = netid
	log.Infof("Assigned id %d to namespace %q", netid, namespaceName)
	return nil
}

func (oc *OsdnController) revokeVNID(namespaceName string) error {
	err := oc.Registry.DeleteNetNamespace(namespaceName)
	if err != nil {
		return err
	}
	netid, found := oc.VNIDMap[namespaceName]
	if !found {
		return fmt.Errorf("Error while fetching Net ID for namespace: %s", namespaceName)
	}
	delete(oc.VNIDMap, namespaceName)

	// Skip AdminVNID as it is not part of Net ID allocation
	if netid == AdminVNID {
		return nil
	}

	// Check if this netid is used by any other namespaces
	// If not, then release the netid
	netid_inuse := false
	for name, id := range oc.VNIDMap {
		if id == netid {
			netid_inuse = true
			log.V(5).Infof("Net ID %d for namespace %q is still in use by namespace %q", netid, namespaceName, name)
			break
		}
	}
	if !netid_inuse {
		err = oc.netIDManager.ReleaseNetID(netid)
		if err != nil {
			return fmt.Errorf("Error while releasing Net ID: %v", err)
		} else {
			log.Infof("Released netid %d for namespace %q", netid, namespaceName)
		}
	}
	return nil
}

func watchNamespaces(oc *OsdnController) {
	nsevent := make(chan *NamespaceEvent)
	stop := make(chan bool)
	go oc.Registry.WatchNamespaces(nsevent, stop)
	for {
		select {
		case ev := <-nsevent:
			switch ev.Type {
			case Added:
				err := oc.assignVNID(ev.Namespace.Name)
				if err != nil {
					log.Errorf("Error assigning Net ID: %v", err)
					continue
				}
			case Deleted:
				err := oc.revokeVNID(ev.Namespace.Name)
				if err != nil {
					log.Errorf("Error revoking Net ID: %v", err)
					continue
				}
			}
		case <-oc.sig:
			log.Error("Signal received. Stopping watching of nodes.")
			stop <- true
			return
		}
	}
}

func (oc *OsdnController) VnidStartNode() error {
	go watchNetNamespaces(oc)
	go watchServices(oc)
	go watchPods(oc)
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
	stop := make(chan bool)
	netNsEvent := make(chan *NetNamespaceEvent)
	go oc.Registry.WatchNetNamespaces(netNsEvent, stop)
	for {
		select {
		case ev := <-netNsEvent:
			switch ev.Type {
			case Added:
				// Skip this event if the old and new network ids are same
				if oldNetID, ok := oc.VNIDMap[ev.NetNamespace.NetName]; ok && (oldNetID == ev.NetNamespace.NetID) {
					continue
				}
				oc.VNIDMap[ev.NetNamespace.Name] = ev.NetNamespace.NetID
				err := oc.updatePodNetwork(ev.NetNamespace.NetName, ev.NetNamespace.NetID)
				if err != nil {
					log.Errorf("Failed to update pod network for namespace '%s', error: %s", ev.NetNamespace.NetName, err)
				}
			case Deleted:
				err := oc.updatePodNetwork(ev.NetNamespace.NetName, AdminVNID)
				if err != nil {
					log.Errorf("Failed to update pod network for namespace '%s', error: %s", ev.NetNamespace.NetName, err)
				}
				delete(oc.VNIDMap, ev.NetNamespace.NetName)
			}
		case <-oc.sig:
			log.Error("Signal received. Stopping watching of NetNamespaces.")
			stop <- true
			return
		}
	}
}

func watchServices(oc *OsdnController) {
	stop := make(chan bool)
	svcevent := make(chan *ServiceEvent)
	go oc.Registry.WatchServices(svcevent, stop)
	for {
		select {
		case ev := <-svcevent:
			var netid uint
			if ev.Type != Deleted {
				var found bool
				netid, found = oc.VNIDMap[ev.Service.Namespace]
				if !found {
					log.Errorf("Error fetching Net ID for namespace: %s, skipped serviceEvent: %v", ev.Service.Namespace, ev)
					continue
				}
			}
			switch ev.Type {
			case Added:
				oc.services[string(ev.Service.UID)] = ev.Service
				oc.pluginHooks.AddServiceRules(ev.Service, netid)
			case Deleted:
				delete(oc.services, string(ev.Service.UID))
				oc.pluginHooks.DeleteServiceRules(ev.Service)
			case Modified:
				oldsvc, exists := oc.services[string(ev.Service.UID)]
				if exists && len(oldsvc.Spec.Ports) == len(ev.Service.Spec.Ports) {
					same := true
					for i := range oldsvc.Spec.Ports {
						if oldsvc.Spec.Ports[i].Protocol != ev.Service.Spec.Ports[i].Protocol || oldsvc.Spec.Ports[i].Port != ev.Service.Spec.Ports[i].Port {
							same = false
							break
						}
					}
					if same {
						continue
					}
				}
				if exists {
					oc.pluginHooks.DeleteServiceRules(oldsvc)
				}
				oc.services[string(ev.Service.UID)] = ev.Service
				oc.pluginHooks.AddServiceRules(ev.Service, netid)
			}
		case <-oc.sig:
			log.Error("Signal received. Stopping watching of services.")
			stop <- true
			return
		}
	}
}

func watchPods(oc *OsdnController) {
	stop := make(chan bool)
	go oc.Registry.WatchPods(stop)

	<-oc.sig
	log.Error("Signal received. Stopping watching of pods.")
	stop <- true
}
