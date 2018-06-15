// +build linux

package node

import (
	"fmt"
	"sync"

	"github.com/golang/glog"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kapihelper "k8s.io/kubernetes/pkg/apis/core/helper"

	"github.com/openshift/origin/pkg/network"
	networkapi "github.com/openshift/origin/pkg/network/apis/network"
)

type multiTenantPlugin struct {
	node  *OsdnNode
	vnids *nodeVNIDMap

	vnidInUseLock sync.Mutex
	vnidInUse     map[uint32]bool
}

func NewMultiTenantPlugin() osdnPolicy {
	return &multiTenantPlugin{
		vnidInUse: make(map[uint32]bool),
	}
}

func (mp *multiTenantPlugin) Name() string {
	return network.MultiTenantPluginName
}

func (mp *multiTenantPlugin) SupportsVNIDs() bool {
	return true
}

func (mp *multiTenantPlugin) Start(node *OsdnNode) error {
	mp.node = node
	mp.vnids = newNodeVNIDMap(mp, node.networkClient)
	if err := mp.vnids.Start(node.networkInformers); err != nil {
		return err
	}

	otx := node.oc.NewTransaction()
	otx.AddFlow("table=80, priority=200, reg0=0, actions=output:NXM_NX_REG2[]")
	otx.AddFlow("table=80, priority=200, reg1=0, actions=output:NXM_NX_REG2[]")
	if err := otx.Commit(); err != nil {
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
		utilruntime.HandleError(fmt.Errorf("Could not get list of local pods in namespace %q: %v", namespace, err))
	}
	services, err := mp.node.kClient.Core().Services(namespace).List(metav1.ListOptions{})
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Could not get list of services in namespace %q: %v", namespace, err))
		services = &kapi.ServiceList{}
	}

	if oldNetID != netID {
		// Update OF rules for the existing/old pods in the namespace
		for _, pod := range pods {
			err = mp.node.UpdatePod(pod)
			if err != nil {
				utilruntime.HandleError(fmt.Errorf("Could not update pod %q in namespace %q: %v", pod.Name, namespace, err))
			}
		}

		// Update OF rules for the old services in the namespace
		for _, svc := range services.Items {
			if !kapihelper.IsServiceIPSet(&svc) {
				continue
			}

			mp.node.DeleteServiceRules(&svc)
			mp.node.AddServiceRules(&svc, netID)
		}

		mp.EnsureVNIDRules(netID)

		// Update namespace references in egress firewall rules
		mp.node.UpdateEgressNetworkPolicyVNID(namespace, oldNetID, netID)
	}

	// Update local multicast rules
	mp.node.podManager.UpdateLocalMulticastRules(oldNetID)
	mp.node.podManager.UpdateLocalMulticastRules(netID)
}

func (mp *multiTenantPlugin) AddNetNamespace(netns *networkapi.NetNamespace) {
	mp.updatePodNetwork(netns.Name, 0, netns.NetID)
}

func (mp *multiTenantPlugin) UpdateNetNamespace(netns *networkapi.NetNamespace, oldNetID uint32) {
	mp.updatePodNetwork(netns.Name, oldNetID, netns.NetID)
}

func (mp *multiTenantPlugin) DeleteNetNamespace(netns *networkapi.NetNamespace) {
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

func (mp *multiTenantPlugin) EnsureVNIDRules(vnid uint32) {
	if vnid == 0 {
		return
	}

	mp.vnidInUseLock.Lock()
	defer mp.vnidInUseLock.Unlock()
	if mp.vnidInUse[vnid] {
		return
	}
	mp.vnidInUse[vnid] = true

	glog.V(5).Infof("EnsureVNIDRules %d - adding rules", vnid)

	otx := mp.node.oc.NewTransaction()
	otx.AddFlow("table=80, priority=100, reg0=%d, reg1=%d, actions=output:NXM_NX_REG2[]", vnid, vnid)
	if err := otx.Commit(); err != nil {
		utilruntime.HandleError(fmt.Errorf("Error adding OVS flow for VNID: %v", err))
	}
}

func (mp *multiTenantPlugin) SyncVNIDRules() {
	mp.vnidInUseLock.Lock()
	defer mp.vnidInUseLock.Unlock()

	unused := mp.node.oc.FindUnusedVNIDs()
	glog.Infof("SyncVNIDRules: %d unused VNIDs", len(unused))

	otx := mp.node.oc.NewTransaction()
	for _, vnid := range unused {
		mp.vnidInUse[uint32(vnid)] = false
		otx.DeleteFlows("table=80, reg1=%d", vnid)
	}
	if err := otx.Commit(); err != nil {
		utilruntime.HandleError(fmt.Errorf("Error deleting syncing OVS VNID rules: %v", err))
	}
}
