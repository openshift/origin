package plugin

import (
	"sync"

	"github.com/golang/glog"

	kapi "k8s.io/kubernetes/pkg/api"

	osapi "github.com/openshift/origin/pkg/sdn/api"
)

type multiTenantPlugin struct {
	node  *OsdnNode
	vnids *nodeVNIDMap

	vnidRefsLock sync.Mutex
	vnidRefs     map[uint32]int
}

func NewMultiTenantPlugin() osdnPolicy {
	return &multiTenantPlugin{
		vnidRefs: make(map[uint32]int),
	}
}

func (mp *multiTenantPlugin) Name() string {
	return osapi.MultiTenantPluginName
}

func (mp *multiTenantPlugin) Start(node *OsdnNode) error {
	mp.node = node
	mp.vnids = newNodeVNIDMap(mp, node.osClient)
	if err := mp.vnids.Start(); err != nil {
		return err
	}

	otx := node.ovs.NewTransaction()
	otx.AddFlow("table=80, priority=200, reg0=0, actions=output:NXM_NX_REG2[]")
	otx.AddFlow("table=80, priority=200, reg1=0, actions=output:NXM_NX_REG2[]")
	if err := otx.EndTransaction(); err != nil {
		return err
	}

	if err := mp.node.SetupEgressNetworkPolicy(); err != nil {
		return err
	}

	return nil
}

func (mp *multiTenantPlugin) updatePodNetwork(namespace string, oldNetID, netID uint32) {
	// FIXME: this is racy; traffic coming from the pods gets switched to the new
	// VNID before the service and firewall rules are updated to match. We need
	// to do the updates as a single transaction (ovs-ofctl --bundle).

	pods, err := mp.node.GetLocalPods(namespace)
	if err != nil {
		glog.Errorf("Could not get list of local pods in namespace %q: %v", namespace, err)
	}
	services, err := mp.node.kClient.Core().Services(namespace).List(kapi.ListOptions{})
	if err != nil {
		glog.Errorf("Could not get list of services in namespace %q: %v", namespace, err)
		services = &kapi.ServiceList{}
	}

	if oldNetID != netID {
		movedVNIDRefs := 0

		// Update OF rules for the existing/old pods in the namespace
		for _, pod := range pods {
			err = mp.node.UpdatePod(pod)
			if err == nil {
				movedVNIDRefs++
			} else {
				glog.Errorf("Could not update pod %q in namespace %q: %v", pod.Name, namespace, err)
			}
		}

		// Update OF rules for the old services in the namespace
		for _, svc := range services.Items {
			if !kapi.IsServiceIPSet(&svc) {
				continue
			}

			mp.node.DeleteServiceRules(&svc)
			mp.node.AddServiceRules(&svc, netID)
			movedVNIDRefs++
		}

		if movedVNIDRefs > 0 {
			mp.moveVNIDRefs(movedVNIDRefs, oldNetID, netID)
		}

		// Update namespace references in egress firewall rules
		mp.node.UpdateEgressNetworkPolicyVNID(namespace, oldNetID, netID)
	}

	// Update local multicast rules
	mp.node.podManager.UpdateLocalMulticastRules(oldNetID)
	mp.node.podManager.UpdateLocalMulticastRules(netID)
}

func (mp *multiTenantPlugin) AddNetNamespace(netns *osapi.NetNamespace) {
	mp.updatePodNetwork(netns.Name, 0, netns.NetID)
}

func (mp *multiTenantPlugin) UpdateNetNamespace(netns *osapi.NetNamespace, oldNetID uint32) {
	mp.updatePodNetwork(netns.Name, oldNetID, netns.NetID)
}

func (mp *multiTenantPlugin) DeleteNetNamespace(netns *osapi.NetNamespace) {
	mp.updatePodNetwork(netns.Name, netns.NetID, 0)
}

func (mp *multiTenantPlugin) GetVNID(namespace string) (uint32, error) {
	return mp.vnids.WaitAndGetVNID(namespace)
}

func (mp *multiTenantPlugin) GetNamespaces(vnid uint32) []string {
	return mp.vnids.GetNamespaces(vnid)
}

func (mp *multiTenantPlugin) GetMulticastEnabled(vnid uint32) bool {
	return mp.vnids.GetMulticastEnabled(vnid)
}

func (mp *multiTenantPlugin) RefVNID(vnid uint32) {
	if vnid == 0 {
		return
	}

	mp.vnidRefsLock.Lock()
	defer mp.vnidRefsLock.Unlock()
	mp.vnidRefs[vnid] += 1
	if mp.vnidRefs[vnid] > 1 {
		return
	}
	glog.V(5).Infof("RefVNID %d adding rule", vnid)

	otx := mp.node.ovs.NewTransaction()
	otx.AddFlow("table=80, priority=100, reg0=%d, reg1=%d, actions=output:NXM_NX_REG2[]", vnid, vnid)
	if err := otx.EndTransaction(); err != nil {
		glog.Errorf("Error adding OVS flow for VNID: %v", err)
	}
}

func (mp *multiTenantPlugin) UnrefVNID(vnid uint32) {
	if vnid == 0 {
		return
	}

	mp.vnidRefsLock.Lock()
	defer mp.vnidRefsLock.Unlock()
	if mp.vnidRefs[vnid] == 0 {
		glog.Warningf("refcounting error on vnid %d", vnid)
		return
	}
	mp.vnidRefs[vnid] -= 1
	if mp.vnidRefs[vnid] > 0 {
		return
	}
	glog.V(5).Infof("UnrefVNID %d removing rule", vnid)

	otx := mp.node.ovs.NewTransaction()
	otx.DeleteFlows("table=80, reg0=%d, reg1=%d", vnid, vnid)
	if err := otx.EndTransaction(); err != nil {
		glog.Errorf("Error deleting OVS flow for VNID: %v", err)
	}
}

func (mp *multiTenantPlugin) moveVNIDRefs(num int, oldVNID, newVNID uint32) {
	glog.V(5).Infof("moveVNIDRefs %d -> %d", oldVNID, newVNID)

	mp.vnidRefsLock.Lock()
	defer mp.vnidRefsLock.Unlock()

	otx := mp.node.ovs.NewTransaction()
	if mp.vnidRefs[oldVNID] <= num {
		otx.DeleteFlows("table=80, reg0=%d, reg1=%d", oldVNID, oldVNID)
	}
	if mp.vnidRefs[newVNID] == 0 {
		otx.AddFlow("table=80, priority=100, reg0=%d, reg1=%d, actions=output:NXM_NX_REG2[]", newVNID, newVNID)
	}
	err := otx.EndTransaction()
	if err != nil {
		glog.Errorf("Error modifying OVS flows for VNID: %v", err)
	}

	mp.vnidRefs[oldVNID] -= num
	if mp.vnidRefs[oldVNID] < 0 {
		glog.Warningf("refcounting error on vnid %d", oldVNID)
		mp.vnidRefs[oldVNID] = 0
	}
	mp.vnidRefs[newVNID] += num
}
