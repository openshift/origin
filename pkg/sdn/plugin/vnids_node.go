package plugin

import (
	"fmt"
	"sync"
	"time"

	log "github.com/golang/glog"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/util/sets"
	utilwait "k8s.io/kubernetes/pkg/util/wait"

	osclient "github.com/openshift/origin/pkg/client"
	osapi "github.com/openshift/origin/pkg/sdn/api"
)

type nodeVNIDMap struct {
	policy   osdnPolicy
	osClient *osclient.Client

	// Synchronizes add or remove ids/namespaces
	lock       sync.Mutex
	ids        map[string]uint32
	mcEnabled  map[string]bool
	namespaces map[uint32]sets.String
}

func newNodeVNIDMap(policy osdnPolicy, osClient *osclient.Client) *nodeVNIDMap {
	return &nodeVNIDMap{
		policy:     policy,
		osClient:   osClient,
		ids:        make(map[string]uint32),
		mcEnabled:  make(map[string]bool),
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
	return 0, fmt.Errorf("failed to find netid for namespace: %s in vnid map", name)
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
		id, err = vmap.GetVNID(name)
		return err == nil, nil
	})
	if err == nil {
		return id, nil
	} else {
		return 0, fmt.Errorf("failed to find netid for namespace: %s in vnid map", name)
	}
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

	log.Infof("Associate netid %d to namespace %q with mcEnabled %v", id, name, mcEnabled)
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
	log.Infof("Dissociate netid %d from namespace %q", id, name)
	return id, nil
}

func netnsIsMulticastEnabled(netns *osapi.NetNamespace) bool {
	enabled, ok := netns.Annotations[osapi.MulticastEnabledAnnotation]
	return enabled == "true" && ok
}

func (vmap *nodeVNIDMap) populateVNIDs() error {
	nets, err := vmap.osClient.NetNamespaces().List(kapi.ListOptions{})
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
	RunEventQueue(vmap.osClient, NetNamespaces, func(delta cache.Delta) error {
		netns := delta.Object.(*osapi.NetNamespace)

		log.V(5).Infof("Watch %s event for NetNamespace %q", delta.Type, netns.ObjectMeta.Name)
		switch delta.Type {
		case cache.Sync, cache.Added, cache.Updated:
			// Skip this event if nothing has changed
			oldNetID, err := vmap.GetVNID(netns.NetName)
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
			vmap.policy.DeleteNetNamespace(netns)
			vmap.unsetVNID(netns.NetName)
		}
		return nil
	})
}
