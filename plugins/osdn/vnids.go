package osdn

import (
	"fmt"

	log "github.com/golang/glog"

	"github.com/openshift/openshift-sdn/pkg/netutils"
	"github.com/openshift/openshift-sdn/plugins/osdn/api"
	osapi "github.com/openshift/origin/pkg/sdn/api"

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

	getNamespaces := func(registry *Registry) (interface{}, string, error) {
		return registry.GetNamespaces()
	}
	result, err := oc.watchAndGetResource("Namespace", watchNamespaces, getNamespaces)
	if err != nil {
		return err
	}

	// 'default' namespace is currently always an admin namespace
	oc.adminNamespaces = append(oc.adminNamespaces, "default")

	// Handle existing namespaces
	namespaces := result.([]string)
	for _, nsName := range namespaces {
		// Revoke invalid VNID for admin namespaces
		if oc.isAdminNamespace(nsName) {
			netid, ok := oc.VNIDMap[nsName]
			if ok && (netid != AdminVNID) {
				err := oc.revokeVNID(nsName)
				if err != nil {
					return err
				}
			}
		}
		_, found := oc.VNIDMap[nsName]
		// Assign VNID for the namespace if it doesn't exist
		if !found {
			err := oc.assignVNID(nsName)
			if err != nil {
				return err
			}
		}
	}

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
	for _, id := range oc.VNIDMap {
		if id == netid {
			netid_inuse = true
			break
		}
	}
	if !netid_inuse {
		err = oc.netIDManager.ReleaseNetID(netid)
		if err != nil {
			return fmt.Errorf("Error while releasing Net ID: %v", err)
		}
	}
	return nil
}

func watchNamespaces(oc *OsdnController, ready chan<- bool, start <-chan string) {
	nsevent := make(chan *api.NamespaceEvent)
	stop := make(chan bool)
	go oc.Registry.WatchNamespaces(nsevent, ready, start, stop)
	for {
		select {
		case ev := <-nsevent:
			switch ev.Type {
			case api.Added:
				err := oc.assignVNID(ev.Namespace.Name)
				if err != nil {
					log.Errorf("Error assigning Net ID: %v", err)
					continue
				}
			case api.Deleted:
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
	getNetNamespaces := func(registry *Registry) (interface{}, string, error) {
		return registry.GetNetNamespaces()
	}
	result, err := oc.watchAndGetResource("NetNamespace", watchNetNamespaces, getNetNamespaces)
	if err != nil {
		return err
	}
	nslist := result.([]osapi.NetNamespace)
	for _, ns := range nslist {
		oc.VNIDMap[ns.Name] = ns.NetID
	}

	getServices := func(registry *Registry) (interface{}, string, error) {
		return registry.GetServices()
	}
	result, err = oc.watchAndGetResource("Service", watchServices, getServices)
	if err != nil {
		return err
	}

	services := result.([]kapi.Service)
	for _, svc := range services {
		netid, found := oc.VNIDMap[svc.Namespace]
		if !found {
			return fmt.Errorf("Error fetching Net ID for namespace: %s", svc.Namespace)
		}
		oc.services[string(svc.UID)] = &svc
		for _, port := range svc.Spec.Ports {
			oc.pluginHooks.AddServiceOFRules(netid, svc.Spec.ClusterIP, port.Protocol, port.Port)
		}
	}

	getPods := func(registry *Registry) (interface{}, string, error) {
		return registry.GetPods()
	}
	_, err = oc.watchAndGetResource("Pod", watchPods, getPods)
	if err != nil {
		return err
	}

	return nil
}

func (oc *OsdnController) updatePodNetwork(namespace string, netID, oldNetID uint) error {
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
		for _, port := range svc.Spec.Ports {
			oc.pluginHooks.DelServiceOFRules(oldNetID, svc.Spec.ClusterIP, port.Protocol, port.Port)
			oc.pluginHooks.AddServiceOFRules(netID, svc.Spec.ClusterIP, port.Protocol, port.Port)
		}
	}
	return nil
}

func watchNetNamespaces(oc *OsdnController, ready chan<- bool, start <-chan string) {
	stop := make(chan bool)
	netNsEvent := make(chan *api.NetNamespaceEvent)
	go oc.Registry.WatchNetNamespaces(netNsEvent, ready, start, stop)
	for {
		select {
		case ev := <-netNsEvent:
			oldNetID, found := oc.VNIDMap[ev.NetNamespace.NetName]
			if !found {
				log.Errorf("Error fetching Net ID for namespace: %s, skipped netNsEvent: %v", ev.NetNamespace.NetName, ev)
			}
			switch ev.Type {
			case api.Added:
				// Skip this event if the old and new network ids are same
				if oldNetID == ev.NetNamespace.NetID {
					continue
				}
				oc.VNIDMap[ev.NetNamespace.Name] = ev.NetNamespace.NetID
				err := oc.updatePodNetwork(ev.NetNamespace.NetName, ev.NetNamespace.NetID, oldNetID)
				if err != nil {
					log.Errorf("Failed to update pod network for namespace '%s', error: %s", ev.NetNamespace.NetName, err)
				}
			case api.Deleted:
				err := oc.updatePodNetwork(ev.NetNamespace.NetName, AdminVNID, oldNetID)
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

func watchServices(oc *OsdnController, ready chan<- bool, start <-chan string) {
	stop := make(chan bool)
	svcevent := make(chan *api.ServiceEvent)
	go oc.Registry.WatchServices(svcevent, ready, start, stop)
	for {
		select {
		case ev := <-svcevent:
			netid, found := oc.VNIDMap[ev.Service.Namespace]
			if !found {
				log.Errorf("Error fetching Net ID for namespace: %s, skipped serviceEvent: %v", ev.Service.Namespace, ev)
			}
			switch ev.Type {
			case api.Added:
				oc.services[string(ev.Service.UID)] = ev.Service
				for _, port := range ev.Service.Spec.Ports {
					oc.pluginHooks.AddServiceOFRules(netid, ev.Service.Spec.ClusterIP, port.Protocol, port.Port)
				}
			case api.Deleted:
				delete(oc.services, string(ev.Service.UID))
				for _, port := range ev.Service.Spec.Ports {
					oc.pluginHooks.DelServiceOFRules(netid, ev.Service.Spec.ClusterIP, port.Protocol, port.Port)
				}
			case api.Modified:
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
					for _, port := range oldsvc.Spec.Ports {
						oc.pluginHooks.DelServiceOFRules(netid, oldsvc.Spec.ClusterIP, port.Protocol, port.Port)
					}
				}
				oc.services[string(ev.Service.UID)] = ev.Service
				for _, port := range ev.Service.Spec.Ports {
					oc.pluginHooks.AddServiceOFRules(netid, ev.Service.Spec.ClusterIP, port.Protocol, port.Port)
				}
			}
		case <-oc.sig:
			log.Error("Signal received. Stopping watching of services.")
			stop <- true
			return
		}
	}
}

func watchPods(oc *OsdnController, ready chan<- bool, start <-chan string) {
	stop := make(chan bool)
	go oc.Registry.WatchPods(ready, start, stop)

	<-oc.sig
	log.Error("Signal received. Stopping watching of pods.")
	stop <- true
}
