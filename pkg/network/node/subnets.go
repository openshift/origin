// +build linux

package node

import (
	"github.com/golang/glog"

	utilwait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"

	networkapi "github.com/openshift/origin/pkg/network/apis/network"
	"github.com/openshift/origin/pkg/network/common"
)

func (node *OsdnNode) SubnetStartNode() error {
	go utilwait.Forever(node.watchSubnets, 0)
	return nil
}

type hostSubnetMap map[string]*networkapi.HostSubnet

func (plugin *OsdnNode) updateVXLANMulticastRules(subnets hostSubnetMap) {
	remoteIPs := make([]string, 0, len(subnets))
	for _, subnet := range subnets {
		if subnet.HostIP != plugin.localIP {
			remoteIPs = append(remoteIPs, subnet.HostIP)
		}
	}
	if err := plugin.oc.UpdateVXLANMulticastFlows(remoteIPs); err != nil {
		glog.Errorf("Error updating OVS VXLAN multicast flows: %v", err)
	}
}

func (node *OsdnNode) watchSubnets() {
	subnets := make(hostSubnetMap)
	common.RunEventQueue(node.networkClient.Network().RESTClient(), common.HostSubnets, func(delta cache.Delta) error {
		hs := delta.Object.(*networkapi.HostSubnet)
		if hs.HostIP == node.localIP {
			return nil
		}

		glog.V(5).Infof("Watch %s event for HostSubnet %q", delta.Type, hs.ObjectMeta.Name)
		switch delta.Type {
		case cache.Sync, cache.Added, cache.Updated:
			oldSubnet, exists := subnets[string(hs.UID)]
			if exists {
				if oldSubnet.HostIP == hs.HostIP {
					break
				} else {
					// Delete old subnet rules
					node.DeleteHostSubnetRules(oldSubnet)
				}
			}
			if err := node.networkInfo.ValidateNodeIP(hs.HostIP); err != nil {
				glog.Warningf("Ignoring invalid subnet for node %s: %v", hs.HostIP, err)
				break
			}

			node.AddHostSubnetRules(hs)
			subnets[string(hs.UID)] = hs
		case cache.Deleted:
			delete(subnets, string(hs.UID))
			node.DeleteHostSubnetRules(hs)
		}
		// Update multicast rules after all other changes have been processed
		node.updateVXLANMulticastRules(subnets)
		return nil
	})
}
