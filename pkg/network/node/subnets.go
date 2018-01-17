// +build linux

package node

import (
	"fmt"
	"time"

	"github.com/golang/glog"

	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func (node *OsdnNode) getLocalSubnet() (string, error) {
	var subnet *networkapi.HostSubnet
	// If the HostSubnet doesn't already exist, it will be created by the SDN master in
	// response to the kubelet registering itself with the master (which should be
	// happening in another goroutine in parallel with this). Sometimes this takes
	// unexpectedly long though, so give it plenty of time before returning an error
	// (since that will cause the node process to exit).
	backoff := utilwait.Backoff{
		// ~2 mins total
		Duration: time.Second,
		Factor:   1.5,
		Steps:    11,
	}
	err := utilwait.ExponentialBackoff(backoff, func() (bool, error) {
		var err error
		subnet, err = node.networkClient.Network().HostSubnets().Get(node.hostName, metav1.GetOptions{})
		if err == nil {
			if subnet.HostIP == node.localIP {
				return true, nil
			} else {
				glog.Warningf("HostIP %q for local subnet does not match with nodeIP %q, "+
					"Waiting for master to update subnet for node %q ...", subnet.HostIP, node.localIP, node.hostName)
				return false, nil
			}
		} else if kapierrors.IsNotFound(err) {
			glog.Warningf("Could not find an allocated subnet for node: %s, Waiting...", node.hostName)
			return false, nil
		} else {
			return false, err
		}
	})
	if err != nil {
		return "", fmt.Errorf("failed to get subnet for this host: %s, error: %v", node.hostName, err)
	}

	if err = node.networkInfo.ValidateNodeIP(subnet.HostIP); err != nil {
		return "", fmt.Errorf("failed to validate own HostSubnet: %v", err)
	}

	return subnet.Subnet, nil
}

func (node *OsdnNode) updateVXLANMulticastRules(subnets hostSubnetMap) {
	remoteIPs := make([]string, 0, len(subnets))
	for _, subnet := range subnets {
		if subnet.HostIP != node.localIP {
			remoteIPs = append(remoteIPs, subnet.HostIP)
		}
	}
	if err := node.oc.UpdateVXLANMulticastFlows(remoteIPs); err != nil {
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
