package plugin

import (
	"fmt"
	"sync"
	"time"

	log "github.com/golang/glog"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/cache"
	kerrors "k8s.io/kubernetes/pkg/util/errors"
	"k8s.io/kubernetes/pkg/util/sets"
	utilwait "k8s.io/kubernetes/pkg/util/wait"

	osclient "github.com/openshift/origin/pkg/client"
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
	var id uint32
	backoff := utilwait.Backoff{
		Duration: 100 * time.Millisecond,
		Factor:   1.5,
		Steps:    5,
	}
	err := utilwait.ExponentialBackoff(backoff, func() (bool, error) {
		var err error
		id, err = vmap.GetVNID(name)
		return err == nil, nil
	})
	if err == nil {
		return id, nil
	} else {
		return 0, fmt.Errorf("Failed to find netid for namespace: %s in vnid map", name)
	}
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

func (vmap *nodeVNIDMap) populateVNIDs(osClient *osclient.Client) error {
	nets, err := osClient.NetNamespaces().List(kapi.ListOptions{})
	if err != nil {
		return err
	}

	for _, net := range nets.Items {
		vmap.setVNID(net.Name, net.NetID)
	}
	return nil
}

//------------------ Node Methods --------------------

func (node *OsdnNode) VnidStartNode() error {
	// Populate vnid map synchronously so that existing services can fetch vnid
	err := node.vnids.populateVNIDs(node.osClient)
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
	services, err := node.kClient.Services(namespace).List(kapi.ListOptions{})
	if err != nil {
		return err
	}

	errList := []error{}

	// Update OF rules for the existing/old pods in the namespace
	for _, pod := range pods {
		err = node.UpdatePod(pod)
		if err != nil {
			errList = append(errList, err)
		}
	}

	// Update OF rules for the old services in the namespace
	for _, svc := range services.Items {
		if !kapi.IsServiceIPSet(&svc) {
			continue
		}

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
	RunEventQueue(node.osClient, NetNamespaces, func(delta cache.Delta) error {
		netns := delta.Object.(*osapi.NetNamespace)

		log.V(5).Infof("Watch %s event for NetNamespace %q", delta.Type, netns.ObjectMeta.Name)
		switch delta.Type {
		case cache.Sync, cache.Added, cache.Updated:
			// Skip this event if the old and new network ids are same
			oldNetID, err := node.vnids.GetVNID(netns.NetName)
			if (err == nil) && (oldNetID == netns.NetID) {
				break
			}
			node.vnids.setVNID(netns.NetName, netns.NetID)

			err = node.updatePodNetwork(netns.NetName, oldNetID, netns.NetID)
			if err != nil {
				node.vnids.setVNID(netns.NetName, oldNetID)
				return fmt.Errorf("failed to update pod network for namespace '%s', error: %s", netns.NetName, err)
			}
		case cache.Deleted:
			// updatePodNetwork needs vnid, so unset vnid after this call
			err := node.updatePodNetwork(netns.NetName, netns.NetID, osapi.GlobalVNID)
			if err != nil {
				return fmt.Errorf("failed to update pod network for namespace '%s', error: %s", netns.NetName, err)
			}
			node.vnids.unsetVNID(netns.NetName)
		}
		return nil
	})
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
	RunEventQueue(node.kClient, Services, func(delta cache.Delta) error {
		serv := delta.Object.(*kapi.Service)

		// Ignore headless services
		if !kapi.IsServiceIPSet(serv) {
			return nil
		}

		log.V(5).Infof("Watch %s event for Service %q", delta.Type, serv.ObjectMeta.Name)
		switch delta.Type {
		case cache.Sync, cache.Added, cache.Updated:
			oldsvc, exists := services[string(serv.UID)]
			if exists {
				if !isServiceChanged(oldsvc, serv) {
					break
				}
				if err := node.DeleteServiceRules(oldsvc); err != nil {
					log.Error(err)
				}
			}

			netid, err := node.vnids.WaitAndGetVNID(serv.Namespace)
			if err != nil {
				return fmt.Errorf("skipped adding service rules for serviceEvent: %v, Error: %v", delta.Type, err)
			}

			if err = node.AddServiceRules(serv, netid); err != nil {
				return err
			}
			services[string(serv.UID)] = serv
		case cache.Deleted:
			delete(services, string(serv.UID))
			if err := node.DeleteServiceRules(serv); err != nil {
				return err
			}
		}
		return nil
	})
}
