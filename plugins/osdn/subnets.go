package osdn

import (
	"fmt"
	"net"
	"time"

	log "github.com/golang/glog"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/types"
	utilruntime "k8s.io/kubernetes/pkg/util/runtime"
	utilwait "k8s.io/kubernetes/pkg/util/wait"
	"k8s.io/kubernetes/pkg/watch"

	"github.com/openshift/openshift-sdn/pkg/netutils"
	osapi "github.com/openshift/origin/pkg/sdn/api"
)

func (oc *OsdnController) SubnetStartMaster(clusterNetwork *net.IPNet, hostSubnetLength uint) error {
	subrange := make([]string, 0)
	subnets, err := oc.Registry.GetSubnets()
	if err != nil {
		log.Errorf("Error in initializing/fetching subnets: %v", err)
		return err
	}
	for _, sub := range subnets {
		subrange = append(subrange, sub.Subnet)
		if err := oc.validateNode(sub.HostIP); err != nil {
			// Don't error out; just warn so the error can be corrected with 'oc'
			log.Errorf("Failed to validate HostSubnet %s: %v", err)
		} else {
			log.Infof("Found existing HostSubnet %s", HostSubnetToString(&sub))
		}
	}

	oc.subnetAllocator, err = netutils.NewSubnetAllocator(clusterNetwork.String(), hostSubnetLength, subrange)
	if err != nil {
		return err
	}

	go utilwait.Forever(oc.watchNodes, 0)
	return nil
}

func (oc *OsdnController) addNode(nodeName string, nodeIP string) error {
	// Validate node IP before proceeding
	if err := oc.validateNode(nodeIP); err != nil {
		return err
	}

	// Check if subnet needs to be created or updated
	subnetCIDR := ""
	sub, err := oc.Registry.GetSubnet(nodeName)
	if err == nil {
		if sub.HostIP == nodeIP {
			return nil
		} else {
			// Node IP changed, delete old subnet
			// TODO: We should add Update REST endpoint for HostSubnet
			err = oc.Registry.DeleteSubnet(nodeName)
			if err != nil {
				return fmt.Errorf("Error deleting subnet for node %s, old ip %s", nodeName, sub.HostIP)
			}
			subnetCIDR = sub.Subnet
		}
	}

	// Get new subnet if needed
	subnetAllocated := false
	if subnetCIDR == "" {
		sn, err := oc.subnetAllocator.GetNetwork()
		if err != nil {
			return fmt.Errorf("Error allocating network for node %s: %v", nodeName, err)
		}
		subnetAllocated = true
		subnetCIDR = sn.String()
	}

	sub, err = oc.Registry.CreateSubnet(nodeName, nodeIP, subnetCIDR)
	if err != nil {
		// Release subnet if needed
		if subnetAllocated {
			_, ipnet, er := net.ParseCIDR(subnetCIDR)
			if er != nil {
				log.Errorf("Failed to release subnet %q for node %q: %v", subnetCIDR, nodeName, er)
			} else {
				oc.subnetAllocator.ReleaseNetwork(ipnet)
			}
		}
		return fmt.Errorf("Error writing subnet %s to etcd for node %s: %v", subnetCIDR, nodeName, err)
	}

	if subnetAllocated {
		log.Infof("Created HostSubnet %s", HostSubnetToString(sub))
	} else {
		log.Infof("Updated HostSubnet %s", HostSubnetToString(sub))
	}
	return nil
}

func (oc *OsdnController) deleteNode(nodeName string) error {
	sub, err := oc.Registry.GetSubnet(nodeName)
	if err != nil {
		return fmt.Errorf("Error fetching subnet for node %q for deletion: %v", nodeName, err)
	}
	_, ipnet, err := net.ParseCIDR(sub.Subnet)
	if err != nil {
		return fmt.Errorf("Error parsing subnet %q for node %q for deletion: %v", sub.Subnet, nodeName, err)
	}
	oc.subnetAllocator.ReleaseNetwork(ipnet)
	err = oc.Registry.DeleteSubnet(nodeName)
	if err != nil {
		return fmt.Errorf("Error deleting subnet %v for node %q: %v", sub, nodeName, err)
	}

	log.Infof("Deleted HostSubnet %s", HostSubnetToString(sub))
	return nil
}

func (oc *OsdnController) SubnetStartNode(mtu uint) (bool, error) {
	err := oc.initSelfSubnet()
	if err != nil {
		return false, err
	}

	// Assume we are working with IPv4
	clusterNetwork, _, servicesNetwork, err := oc.Registry.GetNetworkInfo()
	if err != nil {
		log.Errorf("Failed to obtain ClusterNetwork: %v", err)
		return false, err
	}
	networkChanged, err := oc.pluginHooks.SetupSDN(oc.localSubnet.Subnet, clusterNetwork.String(), servicesNetwork.String(), mtu)
	if err != nil {
		return false, err
	}

	go utilwait.Forever(oc.watchSubnets, 0)
	return networkChanged, nil
}

func (oc *OsdnController) initSelfSubnet() error {
	// timeout: 30 secs
	retries := 60
	retryInterval := 500 * time.Millisecond

	var err error
	var subnet *osapi.HostSubnet
	// Try every retryInterval and bail-out if it exceeds max retries
	for i := 0; i < retries; i++ {
		// Get subnet for current node
		subnet, err = oc.Registry.GetSubnet(oc.HostName)
		if err == nil {
			break
		}
		log.Warningf("Could not find an allocated subnet for node: %s, Waiting...", oc.HostName)
		time.Sleep(retryInterval)
	}
	if err != nil {
		return fmt.Errorf("Failed to get subnet for this host: %s, error: %v", oc.HostName, err)
	}

	if err := oc.validateNode(subnet.HostIP); err != nil {
		return fmt.Errorf("Failed to validate own HostSubnet: %v", err)
	}

	log.Infof("Found local HostSubnet %s", HostSubnetToString(subnet))
	oc.localSubnet = subnet
	return nil
}

// Only run on the master
func (oc *OsdnController) watchNodes() {
	eventQueue := oc.Registry.RunEventQueue(Nodes)
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

		nodeIP, err := GetNodeIP(node)
		if err != nil {
			log.Errorf("Failed to get node IP for %s, skipping event: %v, node: %v", name, eventType, node)
			continue
		}

		switch eventType {
		case watch.Added, watch.Modified:
			if oldNodeIP, ok := nodeAddressMap[uid]; ok && (oldNodeIP == nodeIP) {
				continue
			}

			err = oc.addNode(name, nodeIP)
			if err != nil {
				log.Errorf("Error creating subnet for node %s, ip %s: %v", name, nodeIP, err)
				continue
			}
			nodeAddressMap[uid] = nodeIP
		case watch.Deleted:
			delete(nodeAddressMap, uid)

			err := oc.deleteNode(name)
			if err != nil {
				log.Errorf("Error deleting node %s: %v", name, err)
			}
		}
	}
}

// Only run on the nodes
func (oc *OsdnController) watchSubnets() {
	subnets := make(map[string]*osapi.HostSubnet)
	eventQueue := oc.Registry.RunEventQueue(HostSubnets)

	for {
		eventType, obj, err := eventQueue.Pop()
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("EventQueue failed for subnets: %v", err))
			return
		}
		hs := obj.(*osapi.HostSubnet)

		if hs.HostIP == oc.localIP {
			continue
		}
		switch eventType {
		case watch.Added, watch.Modified:
			oldSubnet, exists := subnets[string(hs.UID)]
			if exists && (oldSubnet.HostIP == hs.HostIP) {
				continue
			}
			if err := oc.validateNode(hs.HostIP); err != nil {
				log.Errorf("Ignoring invalid subnet for node %s: %v", hs.HostIP, err)
				continue
			}

			if err := oc.pluginHooks.AddHostSubnetRules(hs); err != nil {
				log.Error(err)
				continue
			}
			subnets[string(hs.UID)] = hs
		case watch.Deleted:
			delete(subnets, string(hs.UID))

			if err := oc.pluginHooks.DeleteHostSubnetRules(hs); err != nil {
				log.Error(err)
			}
		}
	}
}

func (oc *OsdnController) validateNode(nodeIP string) error {
	if nodeIP == "" || nodeIP == "127.0.0.1" {
		return fmt.Errorf("Invalid node IP %q", nodeIP)
	}

	clusterNet, err := oc.Registry.GetClusterNetwork()
	if err != nil {
		return fmt.Errorf("Failed to get Cluster Network address: %v", err)
	}

	// Ensure each node's NodeIP is not contained by the cluster network,
	// which could cause a routing loop. (rhbz#1295486)
	ipaddr := net.ParseIP(nodeIP)
	if ipaddr == nil {
		return fmt.Errorf("Failed to parse node IP %s", nodeIP)
	}

	if clusterNet.Contains(ipaddr) {
		return fmt.Errorf("Node IP %s conflicts with cluster network %s", nodeIP, clusterNet.String())
	}

	return nil
}
