// +build linux

package node

import (
	"fmt"
	"sync"
	"time"

	"github.com/golang/glog"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	utilwait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"

	networkapi "github.com/openshift/origin/pkg/network/apis/network"
	"github.com/openshift/origin/pkg/network/common"
	networkinformers "github.com/openshift/origin/pkg/network/generated/informers/internalversion"
	networkclient "github.com/openshift/origin/pkg/network/generated/internalclientset"
)

type nodeVNIDMap struct {
	policy           osdnPolicy
	networkClient    networkclient.Interface
	networkInformers networkinformers.SharedInformerFactory

	// Synchronizes add or remove ids/namespaces
	lock       sync.Mutex
	ids        map[string]uint32
	mcEnabled  map[string]bool
	namespaces map[uint32]sets.String
}

func newNodeVNIDMap(policy osdnPolicy, networkClient networkclient.Interface) *nodeVNIDMap {
	return &nodeVNIDMap{
		policy:        policy,
		networkClient: networkClient,
		ids:           make(map[string]uint32),
		mcEnabled:     make(map[string]bool),
		namespaces:    make(map[uint32]sets.String),
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

func (vmap *nodeVNIDMap) GetMulticastEnabled(id uint32) bool {
	vmap.lock.Lock()
	defer vmap.lock.Unlock()

	set, exists := vmap.namespaces[id]
	if !exists || set.Len() == 0 {
		return false
	}
	for _, ns := range set.List() {
		if !vmap.mcEnabled[ns] {
			return false
		}
	}
	return true
}

// Nodes asynchronously watch for both NetNamespaces and services
// NetNamespaces populates vnid map and services/pod-setup depend on vnid map
// If for some reason, vnid map propagation from master to node is slow
// and if service/pod-setup tries to lookup vnid map then it may fail.
// So, use this method to alleviate this problem. This method will
// retry vnid lookup before giving up.
func (vmap *nodeVNIDMap) WaitAndGetVNID(name string) (uint32, error) {
	var id uint32
	// ~5 sec timeout
	backoff := utilwait.Backoff{
		Duration: 400 * time.Millisecond,
		Factor:   1.5,
		Steps:    6,
	}
	err := utilwait.ExponentialBackoff(backoff, func() (bool, error) {
		var err error
		id, err = vmap.getVNID(name)
		return err == nil, nil
	})
	if err == nil {
		return id, nil
	} else {
		// We may find netid when we check with api server but we will
		// still treat this as an error if we don't find it in vnid map.
		// So that we can imply insufficient timeout if we see many VnidNotFoundErrors.
		VnidNotFoundErrors.Inc()

		netns, err := vmap.networkClient.Network().NetNamespaces().Get(name, metav1.GetOptions{})
		if err != nil {
			return 0, fmt.Errorf("failed to find netid for namespace: %s, %v", name, err)
		}
		glog.Warningf("Netid for namespace: %s exists but not found in vnid map", name)
		vmap.setVNID(netns.Name, netns.NetID, netnsIsMulticastEnabled(netns))
		return netns.NetID, nil
	}
}

func (vmap *nodeVNIDMap) getVNID(name string) (uint32, error) {
	vmap.lock.Lock()
	defer vmap.lock.Unlock()

	if id, ok := vmap.ids[name]; ok {
		return id, nil
	}
	return 0, fmt.Errorf("failed to find netid for namespace: %s in vnid map", name)
}

func (vmap *nodeVNIDMap) setVNID(name string, id uint32, mcEnabled bool) {
	vmap.lock.Lock()
	defer vmap.lock.Unlock()

	if oldId, found := vmap.ids[name]; found {
		vmap.removeNamespaceFromSet(name, oldId)
	}
	vmap.ids[name] = id
	vmap.mcEnabled[name] = mcEnabled
	vmap.addNamespaceToSet(name, id)

	glog.Infof("Associate netid %d to namespace %q with mcEnabled %v", id, name, mcEnabled)
}

func (vmap *nodeVNIDMap) unsetVNID(name string) (id uint32, err error) {
	vmap.lock.Lock()
	defer vmap.lock.Unlock()

	id, found := vmap.ids[name]
	if !found {
		return 0, fmt.Errorf("failed to find netid for namespace: %s in vnid map", name)
	}
	vmap.removeNamespaceFromSet(name, id)
	delete(vmap.ids, name)
	delete(vmap.mcEnabled, name)
	glog.Infof("Dissociate netid %d from namespace %q", id, name)
	return id, nil
}

func netnsIsMulticastEnabled(netns *networkapi.NetNamespace) bool {
	enabled, ok := netns.Annotations[networkapi.MulticastEnabledAnnotation]
	return enabled == "true" && ok
}

func (vmap *nodeVNIDMap) populateVNIDs() error {
	nets, err := vmap.networkClient.Network().NetNamespaces().List(metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, net := range nets.Items {
		vmap.setVNID(net.Name, net.NetID, netnsIsMulticastEnabled(&net))
	}
	return nil
}

func (vmap *nodeVNIDMap) Start(networkInformers networkinformers.SharedInformerFactory) error {
	vmap.networkInformers = networkInformers

	// Populate vnid map synchronously so that existing services can fetch vnid
	err := vmap.populateVNIDs()
	if err != nil {
		return err
	}

	vmap.watchNetNamespaces()
	return nil
}

func (vmap *nodeVNIDMap) watchNetNamespaces() {
	funcs := common.InformerFuncs(&networkapi.NetNamespace{}, vmap.handleAddOrUpdateNetNamespace, vmap.handleDeleteNetNamespace)
	vmap.networkInformers.Network().InternalVersion().NetNamespaces().Informer().AddEventHandler(funcs)
}

func (vmap *nodeVNIDMap) handleAddOrUpdateNetNamespace(obj, _ interface{}, eventType watch.EventType) {
	netns := obj.(*networkapi.NetNamespace)
	glog.V(5).Infof("Watch %s event for NetNamespace %q", eventType, netns.Name)

	// Skip this event if nothing has changed
	oldNetID, err := vmap.getVNID(netns.NetName)
	oldMCEnabled := vmap.mcEnabled[netns.NetName]
	mcEnabled := netnsIsMulticastEnabled(netns)
	if err == nil && oldNetID == netns.NetID && oldMCEnabled == mcEnabled {
		return
	}
	vmap.setVNID(netns.NetName, netns.NetID, mcEnabled)

	if eventType == watch.Added {
		vmap.policy.AddNetNamespace(netns)
	} else {
		vmap.policy.UpdateNetNamespace(netns, oldNetID)
	}
}

func (vmap *nodeVNIDMap) handleDeleteNetNamespace(obj interface{}) {
	netns := obj.(*networkapi.NetNamespace)
	glog.V(5).Infof("Watch %s event for NetNamespace %q", watch.Deleted, netns.Name)

	// Unset VNID first so further operations don't see the deleted VNID
	vmap.unsetVNID(netns.NetName)
	vmap.policy.DeleteNetNamespace(netns)
}
