package master

import (
	"fmt"
	"net"
	"strconv"

	"github.com/golang/glog"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	utilwait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/retry"

	"github.com/openshift/origin/pkg/network"
	networkapi "github.com/openshift/origin/pkg/network/apis/network"
	"github.com/openshift/origin/pkg/network/common"
	"github.com/openshift/origin/pkg/util/netutils"
)

func (master *OsdnMaster) SubnetStartMaster(clusterNetworks []common.ClusterNetwork) error {
	subrange := make([]string, 0)
	subnets, err := master.networkClient.Network().HostSubnets().List(metav1.ListOptions{})
	if err != nil {
		glog.Errorf("Error in initializing/fetching subnets: %v", err)
		return err
	}
	for _, sub := range subnets.Items {
		subrange = append(subrange, sub.Subnet)
		if err = master.networkInfo.ValidateNodeIP(sub.HostIP); err != nil {
			// Don't error out; just warn so the error can be corrected with 'oc'
			glog.Errorf("Failed to validate HostSubnet %s: %v", common.HostSubnetToString(&sub), err)
		} else {
			glog.Infof("Found existing HostSubnet %s", common.HostSubnetToString(&sub))
		}
	}
	var subnetAllocatorList []*netutils.SubnetAllocator
	for _, cn := range clusterNetworks {
		subnetAllocator, err := netutils.NewSubnetAllocator(cn.ClusterCIDR.String(), cn.HostSubnetLength, subrange)
		if err != nil {
			return err
		}
		subnetAllocatorList = append(subnetAllocatorList, subnetAllocator)
	}
	master.subnetAllocatorList = subnetAllocatorList

	master.watchNodes()
	go utilwait.Forever(master.watchSubnets, 0)
	return nil
}

// addNode takes the nodeName, a preferred nodeIP, the node's annotations and other valid ip addresses
// Creates or updates a HostSubnet if needed
// Returns the IP address used for hostsubnet (either the preferred or one from the otherValidAddresses) and any error
func (master *OsdnMaster) addNode(nodeName string, nodeIP string, hsAnnotations map[string]string, otherValidAddresses []kapi.NodeAddress) (string, error) {
	// Validate node IP before proceeding
	if err := master.networkInfo.ValidateNodeIP(nodeIP); err != nil {
		return "", err
	}

	// Check if subnet needs to be created or updated
	sub, err := master.networkClient.Network().HostSubnets().Get(nodeName, metav1.GetOptions{})
	if err == nil {
		if sub.HostIP == nodeIP {
			return nodeIP, nil
		} else if isValidNodeIP(otherValidAddresses, sub.HostIP) {
			return sub.HostIP, nil
		} else {
			// Node IP changed, update old subnet
			sub.HostIP = nodeIP
			sub, err = master.networkClient.Network().HostSubnets().Update(sub)
			if err != nil {
				return "", fmt.Errorf("error updating subnet %s for node %s: %v", sub.Subnet, nodeName, err)
			}
			glog.Infof("Updated HostSubnet %s", common.HostSubnetToString(sub))
			return nodeIP, nil
		}
	}

	// Create new subnet
	for _, possibleSubnet := range master.subnetAllocatorList {
		sn, err := possibleSubnet.GetNetwork()
		if err == netutils.ErrSubnetAllocatorFull {
			// Current subnet exhausted, check the next one
			continue
		} else if err != nil {
			glog.Errorf("Error allocating network from subnet: %v", possibleSubnet)
			continue
		} else {
			sub = &networkapi.HostSubnet{
				TypeMeta:   metav1.TypeMeta{Kind: "HostSubnet"},
				ObjectMeta: metav1.ObjectMeta{Name: nodeName, Annotations: hsAnnotations},
				Host:       nodeName,
				HostIP:     nodeIP,
				Subnet:     sn.String(),
			}
			sub, err = master.networkClient.Network().HostSubnets().Create(sub)
			if err != nil {
				possibleSubnet.ReleaseNetwork(sn)
				return "", fmt.Errorf("error allocating node: %s", nodeName)
			}
			glog.Infof("Created HostSubnet %s", common.HostSubnetToString(sub))
			return nodeIP, nil
		}
	}
	return "", fmt.Errorf("error allocating network for node %s: %v", nodeName, err)
}

func (master *OsdnMaster) deleteNode(nodeName string) error {
	sub, err := master.networkClient.Network().HostSubnets().Get(nodeName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("error fetching subnet for node %q for deletion: %v", nodeName, err)
	}
	err = master.networkClient.Network().HostSubnets().Delete(nodeName, &metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("error deleting subnet %v for node %q: %v", sub, nodeName, err)
	}

	glog.Infof("Deleted HostSubnet %s", common.HostSubnetToString(sub))
	return nil
}

func isValidNodeIP(validAddresses []kapi.NodeAddress, nodeIP string) bool {
	for _, addr := range validAddresses {
		if addr.Address == nodeIP {
			return true
		}
	}
	return false
}

func getNodeIP(node *kapi.Node) (string, error) {
	if len(node.Status.Addresses) > 0 && node.Status.Addresses[0].Address != "" {
		return node.Status.Addresses[0].Address, nil
	} else {
		return netutils.GetNodeIP(node.Name)
	}
}

// Because openshift-sdn uses an overlay and doesn't need GCE Routes, we need to
// clear the NetworkUnavailable condition that kubelet adds to initial node
// status when using GCE.
// TODO: make upstream kubelet more flexible with overlays and GCE so this
// condition doesn't get added for network plugins that don't want it, and then
// we can remove this function.
func (master *OsdnMaster) clearInitialNodeNetworkUnavailableCondition(node *kapi.Node) {
	knode := node
	cleared := false
	resultErr := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		var err error

		if knode != node {
			knode, err = master.kClient.Core().Nodes().Get(node.ObjectMeta.Name, metav1.GetOptions{})
			if err != nil {
				return err
			}
		}

		// Let caller modify knode's status, then push to api server.
		_, condition := GetNodeCondition(&node.Status, kapi.NodeNetworkUnavailable)
		if condition != nil && condition.Status != kapi.ConditionFalse && condition.Reason == "NoRouteCreated" {
			condition.Status = kapi.ConditionFalse
			condition.Reason = "RouteCreated"
			condition.Message = "openshift-sdn cleared kubelet-set NoRouteCreated"
			condition.LastTransitionTime = metav1.Now()
			knode, err = master.kClient.Core().Nodes().UpdateStatus(knode)
			if err == nil {
				cleared = true
			}
		}
		return err
	})
	if resultErr != nil {
		utilruntime.HandleError(fmt.Errorf("status update failed for local node: %v", resultErr))
	} else if cleared {
		glog.Infof("Cleared node NetworkUnavailable/NoRouteCreated condition for %s", node.ObjectMeta.Name)
	}
}

// TODO remove this and switch to external
// GetNodeCondition extracts the provided condition from the given status and returns that.
// Returns nil and -1 if the condition is not present, and the index of the located condition.
func GetNodeCondition(status *kapi.NodeStatus, conditionType kapi.NodeConditionType) (int, *kapi.NodeCondition) {
	if status == nil {
		return -1, nil
	}
	for i := range status.Conditions {
		if status.Conditions[i].Type == conditionType {
			return i, &status.Conditions[i]
		}
	}
	return -1, nil
}

func (master *OsdnMaster) watchNodes() {
	common.RegisterSharedInformerEventHandlers(master.informers,
		master.handleAddOrUpdateNode, master.handleDeleteNode, common.Nodes)
}

func (master *OsdnMaster) handleAddOrUpdateNode(obj, _ interface{}, eventType watch.EventType) {
	node := obj.(*kapi.Node)
	nodeIP, err := getNodeIP(node)
	if err != nil {
		glog.Errorf("Failed to get node IP for node %s, skipping %s event, node: %v", node.Name, eventType, node)
		return
	}
	master.clearInitialNodeNetworkUnavailableCondition(node)

	if oldNodeIP, ok := master.hostSubnetNodeIPs[node.UID]; ok && ((nodeIP == oldNodeIP) || isValidNodeIP(node.Status.Addresses, oldNodeIP)) {
		return
	}
	// Node status is frequently updated by kubelet, so log only if the above condition is not met
	glog.V(5).Infof("Watch %s event for Node %q", eventType, node.Name)

	usedNodeIP, err := master.addNode(node.Name, nodeIP, nil, node.Status.Addresses)
	if err != nil {
		glog.Errorf("Error creating subnet for node %s, ip %s: %v", node.Name, nodeIP, err)
		return
	}
	master.hostSubnetNodeIPs[node.UID] = usedNodeIP
}

func (master *OsdnMaster) handleDeleteNode(obj interface{}) {
	node := obj.(*kapi.Node)
	glog.V(5).Infof("Watch %s event for Node %q", watch.Deleted, node.Name)
	delete(master.hostSubnetNodeIPs, node.UID)

	if err := master.deleteNode(node.Name); err != nil {
		glog.Errorf("Error deleting node %s: %v", node.Name, err)
		return
	}
}

// Watch for all hostsubnet events and if one is found with the right annotation, use the SubnetAllocator to dole a real subnet
func (master *OsdnMaster) watchSubnets() {
	common.RunEventQueue(master.networkClient.Network().RESTClient(), common.HostSubnets, func(delta cache.Delta) error {
		hs := delta.Object.(*networkapi.HostSubnet)
		name := hs.ObjectMeta.Name
		hostIP := hs.HostIP
		subnet := hs.Subnet

		glog.V(5).Infof("Watch %s event for HostSubnet %q", delta.Type, hs.ObjectMeta.Name)
		switch delta.Type {
		case cache.Sync, cache.Added, cache.Updated:
			if _, ok := hs.Annotations[networkapi.AssignHostSubnetAnnotation]; ok {
				// Delete the annotated hostsubnet and create a new one with an assigned subnet
				// We do not update (instead of delete+create) because the watchSubnets on the nodes
				// will skip the event if it finds that the hostsubnet has the same host
				// And we cannot fix the watchSubnets code for node because it will break migration if
				// nodes are upgraded after the master
				err := master.networkClient.Network().HostSubnets().Delete(name, &metav1.DeleteOptions{})
				if err != nil {
					glog.Errorf("Error in deleting annotated subnet from master, name: %s, ip %s: %v", name, hostIP, err)
					return nil
				}
				var hsAnnotations map[string]string
				if vnid, ok := hs.Annotations[networkapi.FixedVNIDHostAnnotation]; ok {
					vnidInt, err := strconv.Atoi(vnid)
					if err == nil && vnidInt >= 0 && uint32(vnidInt) <= network.MaxVNID {
						hsAnnotations = make(map[string]string)
						hsAnnotations[networkapi.FixedVNIDHostAnnotation] = strconv.Itoa(vnidInt)
					} else {
						glog.Errorf("Vnid %s is an invalid value for annotation %s. Annotation will be ignored.", vnid, networkapi.FixedVNIDHostAnnotation)
					}
				}
				_, err = master.addNode(name, hostIP, hsAnnotations, nil)
				if err != nil {
					glog.Errorf("Error creating subnet for node %s, ip %s: %v", name, hostIP, err)
					return nil
				}
			}
		case cache.Deleted:
			if _, ok := hs.Annotations[networkapi.AssignHostSubnetAnnotation]; !ok {
				// release the subnet
				_, ipnet, err := net.ParseCIDR(subnet)
				if err != nil {
					return fmt.Errorf("error parsing subnet %q for node %q for deletion: %v", subnet, name, err)
				}
				for _, possibleSubnetAllocator := range master.subnetAllocatorList {
					possibleSubnetAllocator.ReleaseNetwork(ipnet)
				}
			}
		}
		return nil
	})
}
