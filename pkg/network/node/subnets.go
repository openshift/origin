// +build linux

package node

import (
	"fmt"
	"time"

	"github.com/golang/glog"

	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	utilwait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"

	networkapi "github.com/openshift/origin/pkg/network/apis/network"
	"github.com/openshift/origin/pkg/network/common"
)

func (node *OsdnNode) SubnetStartNode() {
	node.watchSubnets()
}

func (node *OsdnNode) watchSubnets() {
	funcs := common.InformerFuncs(&networkapi.HostSubnet{}, node.handleAddOrUpdateHostSubnet, node.handleDeleteHostSubnet)
	node.networkInformers.Network().InternalVersion().HostSubnets().Informer().AddEventHandler(funcs)
}

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

func (node *OsdnNode) handleAddOrUpdateHostSubnet(obj, _ interface{}, eventType watch.EventType) {
	hs := obj.(*networkapi.HostSubnet)
	glog.V(5).Infof("Watch %s event for HostSubnet %q", eventType, hs.Name)

	if hs.HostIP == node.localIP {
		return
	}
	oldSubnet, exists := node.hostSubnetMap[string(hs.UID)]
	if exists {
		if oldSubnet.HostIP == hs.HostIP {
			return
		} else {
			// Delete old subnet rules
			node.DeleteHostSubnetRules(oldSubnet)
		}
	}
	if err := node.networkInfo.ValidateNodeIP(hs.HostIP); err != nil {
		glog.Warningf("Ignoring invalid subnet for node %s: %v", hs.HostIP, err)
		return
	}
	node.AddHostSubnetRules(hs)
	node.hostSubnetMap[string(hs.UID)] = hs

	// Update multicast rules after all other changes have been processed
	node.updateVXLANMulticastRules()
}

func (node *OsdnNode) handleDeleteHostSubnet(obj interface{}) {
	hs := obj.(*networkapi.HostSubnet)
	glog.V(5).Infof("Watch %s event for HostSubnet %q", watch.Deleted, hs.Name)

	if hs.HostIP == node.localIP {
		return
	}
	if _, exists := node.hostSubnetMap[string(hs.UID)]; !exists {
		return
	}

	delete(node.hostSubnetMap, string(hs.UID))
	node.DeleteHostSubnetRules(hs)

	node.updateVXLANMulticastRules()
}

func (node *OsdnNode) updateVXLANMulticastRules() {
	remoteIPs := make([]string, 0, len(node.hostSubnetMap))
	for _, subnet := range node.hostSubnetMap {
		if subnet.HostIP != node.localIP {
			remoteIPs = append(remoteIPs, subnet.HostIP)
		}
	}
	if err := node.oc.UpdateVXLANMulticastFlows(remoteIPs); err != nil {
		utilruntime.HandleError(fmt.Errorf("Error updating OVS VXLAN multicast flows: %v", err))
	}
}
