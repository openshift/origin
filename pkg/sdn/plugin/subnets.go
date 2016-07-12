package plugin

import (
	"fmt"
	"net"
	"strings"
	"time"

	log "github.com/golang/glog"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/types"
	utilruntime "k8s.io/kubernetes/pkg/util/runtime"
	utilwait "k8s.io/kubernetes/pkg/util/wait"
	"k8s.io/kubernetes/pkg/watch"

	osapi "github.com/openshift/origin/pkg/sdn/api"
	"github.com/openshift/origin/pkg/util/netutils"
)

func (master *OsdnMaster) SubnetStartMaster(clusterNetwork *net.IPNet, hostSubnetLength uint32) error {
	subrange := make([]string, 0)
	subnets, err := master.registry.GetSubnets()
	if err != nil {
		log.Errorf("Error in initializing/fetching subnets: %v", err)
		return err
	}
	for _, sub := range subnets {
		subrange = append(subrange, sub.Subnet)
		if err = master.registry.ValidateNodeIP(sub.HostIP); err != nil {
			// Don't error out; just warn so the error can be corrected with 'oc'
			log.Errorf("Failed to validate HostSubnet %s: %v", hostSubnetToString(&sub), err)
		} else {
			log.Infof("Found existing HostSubnet %s", hostSubnetToString(&sub))
		}
	}

	master.subnetAllocator, err = netutils.NewSubnetAllocator(clusterNetwork.String(), hostSubnetLength, subrange)
	if err != nil {
		return err
	}

	go utilwait.Forever(master.watchNodes, 0)
	return nil
}

func (master *OsdnMaster) addNode(nodeName string, nodeIP string) error {
	// Validate node IP before proceeding
	if err := master.registry.ValidateNodeIP(nodeIP); err != nil {
		return err
	}

	// Check if subnet needs to be created or updated
	sub, err := master.registry.GetSubnet(nodeName)
	if err == nil {
		if sub.HostIP == nodeIP {
			return nil
		} else {
			// Node IP changed, update old subnet
			sub.HostIP = nodeIP
			sub, err = master.registry.UpdateSubnet(sub)
			if err != nil {
				return fmt.Errorf("Error updating subnet %s for node %s: %v", sub.Subnet, nodeName, err)
			}
			log.Infof("Updated HostSubnet %s", hostSubnetToString(sub))
			return nil
		}
	}

	// Create new subnet
	sn, err := master.subnetAllocator.GetNetwork()
	if err != nil {
		return fmt.Errorf("Error allocating network for node %s: %v", nodeName, err)
	}

	sub, err = master.registry.CreateSubnet(nodeName, nodeIP, sn.String())
	if err != nil {
		master.subnetAllocator.ReleaseNetwork(sn)
		return fmt.Errorf("Error creating subnet %s for node %s: %v", sn.String(), nodeName, err)
	}
	log.Infof("Created HostSubnet %s", hostSubnetToString(sub))
	return nil
}

func (master *OsdnMaster) deleteNode(nodeName string) error {
	sub, err := master.registry.GetSubnet(nodeName)
	if err != nil {
		return fmt.Errorf("Error fetching subnet for node %q for deletion: %v", nodeName, err)
	}
	_, ipnet, err := net.ParseCIDR(sub.Subnet)
	if err != nil {
		return fmt.Errorf("Error parsing subnet %q for node %q for deletion: %v", sub.Subnet, nodeName, err)
	}
	master.subnetAllocator.ReleaseNetwork(ipnet)
	err = master.registry.DeleteSubnet(nodeName)
	if err != nil {
		return fmt.Errorf("Error deleting subnet %v for node %q: %v", sub, nodeName, err)
	}

	log.Infof("Deleted HostSubnet %s", hostSubnetToString(sub))
	return nil
}

func getNodeIP(node *kapi.Node) (string, error) {
	if len(node.Status.Addresses) > 0 && node.Status.Addresses[0].Address != "" {
		return node.Status.Addresses[0].Address, nil
	} else {
		return netutils.GetNodeIP(node.Name)
	}
}

func (master *OsdnMaster) watchNodes() {
	eventQueue := master.registry.RunEventQueue(Nodes)
	nodeAddressMap := map[types.UID]string{}

	for {
		eventType, obj, err := eventQueue.Pop()
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("EventQueue failed for nodes: %v", err))
			return
		}
		node := obj.(*kapi.Node)
		name := node.ObjectMeta.Name
		uid := node.ObjectMeta.UID

		nodeIP, err := getNodeIP(node)
		if err != nil {
			log.Errorf("Failed to get node IP for %s, skipping event: %v, node: %v", name, eventType, node)
			continue
		}

		switch eventType {
		case watch.Added, watch.Modified:
			if oldNodeIP, ok := nodeAddressMap[uid]; ok && (oldNodeIP == nodeIP) {
				continue
			}
			// Node status is frequently updated by kubelet, so log only if the above condition is not met
			log.V(5).Infof("Watch %s event for Node %q", strings.Title(string(eventType)), name)

			err = master.addNode(name, nodeIP)
			if err != nil {
				log.Errorf("Error creating subnet for node %s, ip %s: %v", name, nodeIP, err)
				continue
			}
			nodeAddressMap[uid] = nodeIP
		case watch.Deleted:
			log.V(5).Infof("Watch %s event for Node %q", strings.Title(string(eventType)), name)
			delete(nodeAddressMap, uid)

			err = master.deleteNode(name)
			if err != nil {
				log.Errorf("Error deleting node %s: %v", name, err)
			}
		}
	}
}

func (node *OsdnNode) SubnetStartNode(mtu uint32) (bool, error) {
	err := node.initSelfSubnet()
	if err != nil {
		return false, err
	}

	// Assume we are working with IPv4
	ni, err := node.registry.GetNetworkInfo()
	if err != nil {
		return false, err
	}
	networkChanged, err := node.SetupSDN(node.localSubnet.Subnet, ni.ClusterNetwork.String(), ni.ServiceNetwork.String(), mtu)
	if err != nil {
		return false, err
	}

	go utilwait.Forever(node.watchSubnets, 0)
	return networkChanged, nil
}

func (node *OsdnNode) initSelfSubnet() error {
	// timeout: 30 secs
	retries := 60
	retryInterval := 500 * time.Millisecond

	var err error
	var subnet *osapi.HostSubnet
	// Try every retryInterval and bail-out if it exceeds max retries
	for i := 0; i < retries; i++ {
		// Get subnet for current node
		subnet, err = node.registry.GetSubnet(node.hostName)
		if err == nil {
			break
		}
		log.Warningf("Could not find an allocated subnet for node: %s, Waiting...", node.hostName)
		time.Sleep(retryInterval)
	}
	if err != nil {
		return fmt.Errorf("Failed to get subnet for this host: %s, error: %v", node.hostName, err)
	}

	if err = node.registry.ValidateNodeIP(subnet.HostIP); err != nil {
		return fmt.Errorf("Failed to validate own HostSubnet: %v", err)
	}

	log.Infof("Found local HostSubnet %s", hostSubnetToString(subnet))
	node.localSubnet = subnet
	return nil
}

// Only run on the nodes
func (node *OsdnNode) watchSubnets() {
	subnets := make(map[string]*osapi.HostSubnet)
	eventQueue := node.registry.RunEventQueue(HostSubnets)

	for {
		eventType, obj, err := eventQueue.Pop()
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("EventQueue failed for subnets: %v", err))
			return
		}
		hs := obj.(*osapi.HostSubnet)

		if hs.HostIP == node.localIP {
			continue
		}

		log.V(5).Infof("Watch %s event for HostSubnet %q", strings.Title(string(eventType)), hs.ObjectMeta.Name)
		switch eventType {
		case watch.Added, watch.Modified:
			oldSubnet, exists := subnets[string(hs.UID)]
			if exists {
				if oldSubnet.HostIP == hs.HostIP {
					continue
				} else {
					// Delete old subnet rules
					if err = node.DeleteHostSubnetRules(oldSubnet); err != nil {
						log.Error(err)
					}
				}
			}
			if err = node.registry.ValidateNodeIP(hs.HostIP); err != nil {
				log.Errorf("Ignoring invalid subnet for node %s: %v", hs.HostIP, err)
				continue
			}

			if err = node.AddHostSubnetRules(hs); err != nil {
				log.Error(err)
				continue
			}
			subnets[string(hs.UID)] = hs
		case watch.Deleted:
			delete(subnets, string(hs.UID))

			if err = node.DeleteHostSubnetRules(hs); err != nil {
				log.Error(err)
			}
		}
	}
}
