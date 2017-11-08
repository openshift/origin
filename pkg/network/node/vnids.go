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
	"k8s.io/client-go/tools/cache"

	networkapi "github.com/openshift/origin/pkg/network/apis/network"
	"github.com/openshift/origin/pkg/network/common"
	networkclient "github.com/openshift/origin/pkg/network/generated/internalclientset"
)

type nodeVNIDMap struct {
	policy        osdnPolicy
	networkClient networkclient.Interface

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
	backoff := utilwait.Backoff{
		Duration: 100 * time.Millisecond,
		Factor:   1.5,
		Steps:    5,
	}
	err := utilwait.ExponentialBackoff(backoff, func() (bool, error) {
		var err error
		id, err = vmap.getVNID(name)
		return err == nil, nil
	})
	if err == nil {
		return id, nil
	} else {
		VnidNotFoundErrors.Inc()
		return 0, fmt.Errorf("failed to find netid for namespace: %s in vnid map", name)
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

func (vmap *nodeVNIDMap) Start() error {
	// Populate vnid map synchronously so that existing services can fetch vnid
	err := vmap.populateVNIDs()
	if err != nil {
		return err
	}

	go utilwait.Forever(vmap.watchNetNamespaces, 0)
	return nil
}

func (vmap *nodeVNIDMap) watchNetNamespaces() {
	common.RunEventQueue(vmap.networkClient.Network().RESTClient(), common.NetNamespaces, func(delta cache.Delta) error {
		netns := delta.Object.(*networkapi.NetNamespace)

		glog.V(5).Infof("Watch %s event for NetNamespace %q", delta.Type, netns.ObjectMeta.Name)
		switch delta.Type {
		case cache.Sync, cache.Added, cache.Updated:
			// Skip this event if nothing has changed
			oldNetID, err := vmap.getVNID(netns.NetName)
			oldMCEnabled := vmap.mcEnabled[netns.NetName]
			mcEnabled := netnsIsMulticastEnabled(netns)
			if err == nil && oldNetID == netns.NetID && oldMCEnabled == mcEnabled {
				break
			}
			vmap.setVNID(netns.NetName, netns.NetID, mcEnabled)

			if delta.Type == cache.Added {
				vmap.policy.AddNetNamespace(netns)
			} else {
				vmap.policy.UpdateNetNamespace(netns, oldNetID)
			}
		case cache.Deleted:
			// Unset VNID first so further operations don't see the deleted VNID
			vmap.unsetVNID(netns.NetName)
			vmap.policy.DeleteNetNamespace(netns)
		}
		return nil
	})
}
