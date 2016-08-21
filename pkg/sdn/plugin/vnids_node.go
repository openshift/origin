package plugin

import (
	"fmt"
	"strings"
	"sync"
	"time"

	log "github.com/golang/glog"

	kapi "k8s.io/kubernetes/pkg/api"
	kubetypes "k8s.io/kubernetes/pkg/kubelet/container"
	kerrors "k8s.io/kubernetes/pkg/util/errors"
	utilruntime "k8s.io/kubernetes/pkg/util/runtime"
	"k8s.io/kubernetes/pkg/util/sets"
	utilwait "k8s.io/kubernetes/pkg/util/wait"
	"k8s.io/kubernetes/pkg/watch"

	osapi "github.com/openshift/origin/pkg/sdn/api"
)

type nodeVNIDMap struct {
	// Synchronizes add or remove ids/namespaces
	lock       sync.Mutex
	ids        map[string]uint32
	namespaces map[uint32]sets.String
}

func newNodeVNIDMap() *nodeVNIDMap {
	return &nodeVNIDMap{
		ids:        make(map[string]uint32),
		namespaces: make(map[uint32]sets.String),
	}
}

func (vmap *nodeVNIDMap) addNamespaceToSet(name string, vnid uint32) {
	set, found := vmap.namespaces[vnid]
	if !found {
		set = sets.NewString()
		vmap.namespaces[vnid] = set
	}
	set.Insert(name)
}

func (vmap *nodeVNIDMap) removeNamespaceFromSet(name string, vnid uint32) {
	if set, found := vmap.namespaces[vnid]; found {
		set.Delete(name)
		if set.Len() == 0 {
			delete(vmap.namespaces, vnid)
		}
	}
}

func (vmap *nodeVNIDMap) GetNamespaces(id uint32) []string {
	vmap.lock.Lock()
	defer vmap.lock.Unlock()

	if set, ok := vmap.namespaces[id]; ok {
		return set.List()
	} else {
		return nil
	}
}

func (vmap *nodeVNIDMap) GetVNID(name string) (uint32, error) {
	vmap.lock.Lock()
	defer vmap.lock.Unlock()

	if id, ok := vmap.ids[name]; ok {
		return id, nil
	}
	return 0, fmt.Errorf("Failed to find netid for namespace: %s in vnid map", name)
}

// Nodes asynchronously watch for both NetNamespaces and services
// NetNamespaces populates vnid map and services/pod-setup depend on vnid map
// If for some reason, vnid map propagation from master to node is slow
// and if service/pod-setup tries to lookup vnid map then it may fail.
// So, use this method to alleviate this problem. This method will
// retry vnid lookup before giving up.
func (vmap *nodeVNIDMap) WaitAndGetVNID(name string) (uint32, error) {
	// Try few times up to 2 seconds
	retries := 20
	retryInterval := 100 * time.Millisecond
	for i := 0; i < retries; i++ {
		if id, err := vmap.GetVNID(name); err == nil {
			return id, nil
		}
		time.Sleep(retryInterval)
	}

	return 0, fmt.Errorf("Failed to find netid for namespace: %s in vnid map", name)
}

func (vmap *nodeVNIDMap) setVNID(name string, id uint32) {
	vmap.lock.Lock()
	defer vmap.lock.Unlock()

	if oldId, found := vmap.ids[name]; found {
		vmap.removeNamespaceFromSet(name, oldId)
	}
	vmap.ids[name] = id
	vmap.addNamespaceToSet(name, id)

	log.Infof("Associate netid %d to namespace %q", id, name)
}

func (vmap *nodeVNIDMap) unsetVNID(name string) (id uint32, err error) {
	vmap.lock.Lock()
	defer vmap.lock.Unlock()

	id, found := vmap.ids[name]
	if !found {
		return 0, fmt.Errorf("Failed to find netid for namespace: %s in vnid map", name)
	}
	vmap.removeNamespaceFromSet(name, id)
	delete(vmap.ids, name)
	log.Infof("Dissociate netid %d from namespace %q", id, name)
	return id, nil
}

func (vmap *nodeVNIDMap) populateVNIDs(registry *Registry) error {
	nets, err := registry.GetNetNamespaces()
	if err != nil {
		return err
	}

	for _, net := range nets {
		vmap.setVNID(net.Name, net.NetID)
	}
	return nil
}

//------------------ Node Methods --------------------

func (node *OsdnNode) VnidStartNode() error {
	// Populate vnid map synchronously so that existing services can fetch vnid
	err := node.vnids.populateVNIDs(node.registry)
	if err != nil {
		return err
	}

	go utilwait.Forever(node.watchNetNamespaces, 0)
	go utilwait.Forever(node.watchServices, 0)
	return nil
}

func (node *OsdnNode) updatePodNetwork(namespace string, oldNetID, netID uint32) error {
	// FIXME: this is racy; traffic coming from the pods gets switched to the new
	// VNID before the service and firewall rules are updated to match. We need
	// to do the updates as a single transaction (ovs-ofctl --bundle).

	pods, err := node.GetLocalPods(namespace)
	if err != nil {
		return err
	}
	services, err := node.registry.GetServicesForNamespace(namespace)
	if err != nil {
		return err
	}

	errList := []error{}

	// Update OF rules for the existing/old pods in the namespace
	for _, pod := range pods {
		err = node.UpdatePod(pod.Namespace, pod.Name, kubetypes.DockerID(getPodContainerID(&pod)))
		if err != nil {
			errList = append(errList, err)
		}
	}

	// Update OF rules for the old services in the namespace
	for _, svc := range services {
		if err = node.DeleteServiceRules(&svc); err != nil {
			log.Error(err)
		}
		if err = node.AddServiceRules(&svc, netID); err != nil {
			errList = append(errList, err)
		}
	}

	// Update namespace references in egress firewall rules
	if err = node.UpdateEgressNetworkPolicyVNID(namespace, oldNetID, netID); err != nil {
		errList = append(errList, err)
	}

	return kerrors.NewAggregate(errList)
}

func (node *OsdnNode) watchNetNamespaces() {
	eventQueue := node.registry.RunEventQueue(NetNamespaces)

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
			var oldNetID uint32
			oldNetID, err = node.vnids.GetVNID(netns.NetName)
			if (err == nil) && (oldNetID == netns.NetID) {
				continue
			}
			node.vnids.setVNID(netns.NetName, netns.NetID)

			err = node.updatePodNetwork(netns.NetName, oldNetID, netns.NetID)
			if err != nil {
				log.Errorf("Failed to update pod network for namespace '%s', error: %s", netns.NetName, err)
				node.vnids.setVNID(netns.NetName, oldNetID)
				continue
			}
		case watch.Deleted:
			// updatePodNetwork needs vnid, so unset vnid after this call
			err = node.updatePodNetwork(netns.NetName, netns.NetID, osapi.GlobalVNID)
			if err != nil {
				log.Errorf("Failed to update pod network for namespace '%s', error: %s", netns.NetName, err)
			}
			node.vnids.unsetVNID(netns.NetName)
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

func (node *OsdnNode) watchServices() {
	services := make(map[string]*kapi.Service)
	eventQueue := node.registry.RunEventQueue(Services)

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
				if err = node.DeleteServiceRules(oldsvc); err != nil {
					log.Error(err)
				}
			}

			var netid uint32
			netid, err = node.vnids.WaitAndGetVNID(serv.Namespace)
			if err != nil {
				log.Errorf("Skipped adding service rules for serviceEvent: %v, Error: %v", eventType, err)
				continue
			}

			if err = node.AddServiceRules(serv, netid); err != nil {
				log.Error(err)
				continue
			}
			services[string(serv.UID)] = serv
		case watch.Deleted:
			delete(services, string(serv.UID))

			if err = node.DeleteServiceRules(serv); err != nil {
				log.Error(err)
			}
		}
	}
}
