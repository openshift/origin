package plugin

import (
	"github.com/golang/glog"

	kapi "k8s.io/kubernetes/pkg/api"

	osapi "github.com/openshift/origin/pkg/sdn/api"
)

type multiTenantPlugin struct {
	node  *OsdnNode
	vnids *nodeVNIDMap
}

func NewMultiTenantPlugin() osdnPolicy {
	return &multiTenantPlugin{}
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

	// Update OF rules for the existing/old pods in the namespace
	for _, pod := range pods {
		err = mp.node.UpdatePod(pod)
		if err != nil {
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
	}

	// Update namespace references in egress firewall rules
	mp.node.UpdateEgressNetworkPolicyVNID(namespace, oldNetID, netID)
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
