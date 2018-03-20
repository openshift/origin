package master

import (
	"fmt"
	"net"
	"strconv"

	"github.com/golang/glog"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/util/retry"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	"github.com/openshift/origin/pkg/network"
	networkapi "github.com/openshift/origin/pkg/network/apis/network"
	"github.com/openshift/origin/pkg/network/common"
)

func (master *OsdnMaster) SubnetStartMaster(clusterNetworks []common.ClusterNetwork) error {
	subrange := make(map[common.ClusterNetwork][]string)
	subnets, err := master.networkClient.Network().HostSubnets().List(metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("error in initializing/fetching subnets: %v", err)
	}
	for _, sub := range subnets.Items {
		if err = master.networkInfo.ValidateNodeIP(sub.HostIP); err != nil {
			// Don't error out; just warn so the error can be corrected with 'oc'
			utilruntime.HandleError(fmt.Errorf("Failed to validate HostSubnet %s: %v", common.HostSubnetToString(&sub), err))
		} else {
			glog.Infof("Found existing HostSubnet %s", common.HostSubnetToString(&sub))
			_, subnetIP, err := net.ParseCIDR(sub.Subnet)
			if err != nil {
				return fmt.Errorf("failed to parse network address: %q", sub.Subnet)
			}

			for _, cn := range clusterNetworks {
				if cn.ClusterCIDR.Contains(subnetIP.IP) {
					subrange[cn] = append(subrange[cn], sub.Subnet)
					break
				}
			}
		}
	}
	var subnetAllocatorList []*SubnetAllocator
	for _, cn := range clusterNetworks {
		subnetAllocator, err := NewSubnetAllocator(cn.ClusterCIDR.String(), cn.HostSubnetLength, subrange[cn])
		if err != nil {
			return err
		}
		subnetAllocatorList = append(subnetAllocatorList, subnetAllocator)
	}
	master.subnetAllocatorList = subnetAllocatorList

	err = master.reconcileHostSubnets(subnets.Items)
	if err != nil {
		return err
	}

	master.watchNodes()
	master.watchSubnets()
	return nil
}

// reconcileHostSubnets verifies and corrects the state of the hostsubnets on startup of the master.
// Because openshift watches on events to keep hostsubnets and nodes in the correct state, missing an event
// can cause orphaned or unusable hostsubnets to stick around.
func (master *OsdnMaster) reconcileHostSubnets(hostSubnets []networkapi.HostSubnet) error {
	nodes, err := master.kClient.Core().Nodes().List(metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("error in initializing/fetching nodes: %v", err)
	}
	nodeUIDmap := make(map[string]string)
	for _, node := range nodes.Items {
		nodeUIDmap[node.Name] = string(node.UID)
	}
	for _, subnet := range hostSubnets {
		if len(nodeUIDmap[subnet.Name]) == 0 && len(subnet.Annotations[networkapi.NodeUIDAnnotation]) == 0 {
			// Subnet belongs to F5, Ignore.
			continue
		} else if len(nodeUIDmap[subnet.Name]) > 0 && len(subnet.Annotations[networkapi.NodeUIDAnnotation]) == 0 {
			// Update path, stamp UID annotation on subnet.
			if subnet.Annotations == nil {
				subnet.Annotations = make(map[string]string)
			}
			subnet.Annotations[networkapi.NodeUIDAnnotation] = nodeUIDmap[subnet.Name]
			_, err := master.networkClient.Network().HostSubnets().Update(&subnet)
			if err != nil {
				return fmt.Errorf("error updating subnet %v for node %s: %v", subnet, subnet.Name, err)
			}
		} else if len(nodeUIDmap[subnet.Name]) == 0 && len(subnet.Annotations[networkapi.NodeUIDAnnotation]) > 0 {
			// Missed Node event, delete stale subnet.
			glog.Infof("Setup found no node associated with hostsubnet %s, deleting the hostsubnet", subnet.Name)
			err := master.networkClient.Network().HostSubnets().Delete(subnet.Name, &metav1.DeleteOptions{})
			if err != nil {
				return fmt.Errorf("error deleting subnet %v: %v", subnet, err)
			}
		} else if nodeUIDmap[subnet.Name] != subnet.Annotations[networkapi.NodeUIDAnnotation] {
			// Missed Node event, node with the same name exists delete stale subnet.
			glog.Infof("Missed node event, hostsubnet %s has the UID of an incorrect object, deleting the hostsubnet", subnet.Name)
			err := master.networkClient.Network().HostSubnets().Delete(subnet.Name, &metav1.DeleteOptions{})
			if err != nil {
				return fmt.Errorf("error deleting subnet %v: %v", subnet, err)
			}
		}
	}
	return nil
}

// addNode takes the nodeName, a preferred nodeIP and the node's annotations
// Creates or updates a HostSubnet if needed
// Returns the IP address used for hostsubnet (either the preferred or one from the otherValidAddresses) and any error
func (master *OsdnMaster) addNode(nodeName string, nodeUID string, nodeIP string, hsAnnotations map[string]string) (string, error) {
	// Validate node IP before proceeding
	if err := master.networkInfo.ValidateNodeIP(nodeIP); err != nil {
		return "", err
	}

	// Check if subnet needs to be created or updated
	sub, err := master.networkClient.Network().HostSubnets().Get(nodeName, metav1.GetOptions{})
	if err == nil {
		if sub.HostIP == nodeIP {
			return nodeIP, nil
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
	if len(nodeUID) != 0 {
		if hsAnnotations == nil {
			hsAnnotations = make(map[string]string)
		}
		hsAnnotations[networkapi.NodeUIDAnnotation] = nodeUID
	}
	for _, possibleSubnet := range master.subnetAllocatorList {
		sn, err := possibleSubnet.GetNetwork()
		if err == ErrSubnetAllocatorFull {
			// Current subnet exhausted, check the next one
			continue
		} else if err != nil {
			utilruntime.HandleError(fmt.Errorf("Error allocating network from subnet: %v", possibleSubnet))
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

// Because openshift-sdn uses an overlay and doesn't need GCE Routes, we need to
// clear the NetworkUnavailable condition that kubelet adds to initial node
// status when using GCE.
// TODO: make upstream kubelet more flexible with overlays and GCE so this
// condition doesn't get added for network plugins that don't want it, and then
// we can remove this function.
func (master *OsdnMaster) clearInitialNodeNetworkUnavailableCondition(origNode *kapi.Node) {
	// Informer cache should not be mutated, so get a copy of the object
	node := origNode.DeepCopy()
	knode := node
	cleared := false
	resultErr := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		var err error

		if knode != node {
			knode, err = master.kClient.Core().Nodes().Get(node.Name, metav1.GetOptions{})
			if err != nil {
				return err
			}
		}

		for i := range knode.Status.Conditions {
			if knode.Status.Conditions[i].Type == kapi.NodeNetworkUnavailable {
				condition := &knode.Status.Conditions[i]
				if condition.Status != kapi.ConditionFalse && condition.Reason == "NoRouteCreated" {
					condition.Status = kapi.ConditionFalse
					condition.Reason = "RouteCreated"
					condition.Message = "openshift-sdn cleared kubelet-set NoRouteCreated"
					condition.LastTransitionTime = metav1.Now()

					if knode, err = master.kClient.Core().Nodes().UpdateStatus(knode); err == nil {
						cleared = true
					}
				}
				break
			}
		}
		return err
	})
	if resultErr != nil {
		utilruntime.HandleError(fmt.Errorf("status update failed for local node: %v", resultErr))
	} else if cleared {
		glog.Infof("Cleared node NetworkUnavailable/NoRouteCreated condition for %s", node.Name)
	}
}

func (master *OsdnMaster) watchNodes() {
	funcs := common.InformerFuncs(&kapi.Node{}, master.handleAddOrUpdateNode, master.handleDeleteNode)
	master.kubeInformers.Core().InternalVersion().Nodes().Informer().AddEventHandler(funcs)
}

func (master *OsdnMaster) handleAddOrUpdateNode(obj, _ interface{}, eventType watch.EventType) {
	node := obj.(*kapi.Node)

	var nodeIP string
	for _, addr := range node.Status.Addresses {
		if addr.Type == kapi.NodeInternalIP {
			nodeIP = addr.Address
			break
		}
	}
	if len(nodeIP) == 0 {
		utilruntime.HandleError(fmt.Errorf("Node IP is not set for node %s, skipping %s event, node: %v", node.Name, eventType, node))
		return
	}

	if oldNodeIP, ok := master.hostSubnetNodeIPs[node.UID]; ok && (nodeIP == oldNodeIP) {
		return
	}
	// Node status is frequently updated by kubelet, so log only if the above condition is not met
	glog.V(5).Infof("Watch %s event for Node %q", eventType, node.Name)

	master.clearInitialNodeNetworkUnavailableCondition(node)

	usedNodeIP, err := master.addNode(node.Name, string(node.UID), nodeIP, nil)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Error creating subnet for node %s, ip %s: %v", node.Name, nodeIP, err))
		return
	}
	master.hostSubnetNodeIPs[node.UID] = usedNodeIP
}

func (master *OsdnMaster) handleDeleteNode(obj interface{}) {
	node := obj.(*kapi.Node)
	glog.V(5).Infof("Watch %s event for Node %q", watch.Deleted, node.Name)
	delete(master.hostSubnetNodeIPs, node.UID)

	if err := master.deleteNode(node.Name); err != nil {
		utilruntime.HandleError(fmt.Errorf("Error deleting node %s: %v", node.Name, err))
		return
	}
}

// Watch for all hostsubnet events and if one is found with the right annotation, use the SubnetAllocator to dole a real subnet
// This is mainly to handle F5 use case, allocate/release subnet with no real node in the cluster.
// - Admin manually creates HostSubnet with 'AssignHostSubnetAnnotation' to allocate a subnet.
// - Admin manually deletes HostSubnet to release the allocated subnet because there won't be
//   node deletion event to trigger HostSubnet deletion in this case.
func (master *OsdnMaster) watchSubnets() {
	funcs := common.InformerFuncs(&networkapi.HostSubnet{}, master.handleAddOrUpdateSubnet, master.handleDeleteSubnet)
	master.networkInformers.Network().InternalVersion().HostSubnets().Informer().AddEventHandler(funcs)
}

func (master *OsdnMaster) handleAddOrUpdateSubnet(obj, _ interface{}, eventType watch.EventType) {
	hs := obj.(*networkapi.HostSubnet)
	glog.V(5).Infof("Watch %s event for HostSubnet %q", eventType, hs.Name)

	if _, ok := hs.Annotations[networkapi.AssignHostSubnetAnnotation]; !ok {
		return
	}

	// Delete the annotated hostsubnet and create a new one with an assigned subnet
	// We do not update (instead of delete+create) because the watchSubnets on the nodes
	// will skip the event if it finds that the hostsubnet has the same host
	// And we cannot fix the watchSubnets code for node because it will break migration if
	// nodes are upgraded after the master
	err := master.networkClient.Network().HostSubnets().Delete(hs.Name, &metav1.DeleteOptions{})
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Error in deleting annotated subnet from master, name: %s, ip %s: %v", hs.Name, hs.HostIP, err))
		return
	}
	var hsAnnotations map[string]string
	if vnid, ok := hs.Annotations[networkapi.FixedVNIDHostAnnotation]; ok {
		vnidInt, err := strconv.Atoi(vnid)
		if err == nil && vnidInt >= 0 && uint32(vnidInt) <= network.MaxVNID {
			hsAnnotations = make(map[string]string)
			hsAnnotations[networkapi.FixedVNIDHostAnnotation] = strconv.Itoa(vnidInt)
		} else {
			utilruntime.HandleError(fmt.Errorf("VNID %s is an invalid value for annotation %s. Annotation will be ignored.", vnid, networkapi.FixedVNIDHostAnnotation))
		}
	}
	_, err = master.addNode(hs.Name, "", hs.HostIP, hsAnnotations)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Error creating subnet for node %s, ip %s: %v", hs.Name, hs.HostIP, err))
		return
	}
}

func (master *OsdnMaster) handleDeleteSubnet(obj interface{}) {
	hs := obj.(*networkapi.HostSubnet)
	glog.V(5).Infof("Watch %s event for HostSubnet %q", watch.Deleted, hs.Name)

	if _, ok := hs.Annotations[networkapi.AssignHostSubnetAnnotation]; ok {
		return
	}

	// release the subnet
	_, ipnet, err := net.ParseCIDR(hs.Subnet)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Error parsing subnet %q for node %q for deletion: %v", hs.Subnet, hs.Name, err))
		return
	}
	for _, possibleSubnetAllocator := range master.subnetAllocatorList {
		possibleSubnetAllocator.ReleaseNetwork(ipnet)
	}
}
