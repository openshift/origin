package osdn

import (
	"fmt"
	"net"
	"time"

	log "github.com/golang/glog"

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

	go watchNodes(oc)
	return nil
}

func (oc *OsdnController) addNode(nodeName string, nodeIP string) error {
	if nodeIP == "" || nodeIP == "127.0.0.1" {
		return fmt.Errorf("Invalid node IP %q for node %q", nodeIP, nodeName)
	}

	sn, err := oc.subnetAllocator.GetNetwork()
	if err != nil {
		return fmt.Errorf("Error allocating network for node %s: %v", nodeName, err)
	}
	sub, err := oc.Registry.CreateSubnet(nodeName, nodeIP, sn.String())
	if err != nil {
		return fmt.Errorf("Error writing subnet %v to etcd for node %s: %v", sn, nodeName, err)
	}

	log.Infof("Created HostSubnet %s", HostSubnetToString(sub))
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

	go watchSubnets(oc)
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
func watchNodes(oc *OsdnController) {
	nodeEvent := make(chan *NodeEvent)
	go oc.Registry.WatchNodes(nodeEvent)
	for {
		ev := <-nodeEvent
		switch ev.Type {
		case Added:
			nodeIP, nodeErr := GetNodeIP(ev.Node)
			if nodeErr == nil {
				nodeErr = oc.validateNode(nodeIP)
			}

			sub, err := oc.Registry.GetSubnet(ev.Node.Name)
			if err != nil {
				if nodeErr == nil {
					// subnet does not exist already
					err = oc.addNode(ev.Node.Name, nodeIP)
					if err != nil {
						log.Errorf("Error adding node: %v", err)
						continue
					}
				} else {
					log.Errorf("Ignoring invalid node %s/%s: %v", ev.Node.Name, nodeIP, nodeErr)
				}
			} else {
				if sub.HostIP != nodeIP {
					err = oc.Registry.DeleteSubnet(ev.Node.Name)
					if err != nil {
						log.Errorf("Error deleting subnet for node %s, old ip %s: %v", ev.Node.Name, sub.HostIP, err)
						continue
					}
					if nodeErr == nil {
						_, err := oc.Registry.CreateSubnet(ev.Node.Name, nodeIP, sub.Subnet)
						if err != nil {
							log.Errorf("Error creating subnet for node %s, ip %s: %v", ev.Node.Name, sub.HostIP, err)
							continue
						}
						log.Infof("Updated HostSubnet %s to HostIP %s", HostSubnetToString(sub), nodeIP)
					} else {
						log.Errorf("Ignoring creating invalid node %s/%s: %v", ev.Node.Name, nodeIP, nodeErr)
					}
				}
			}
		case Deleted:
			err := oc.deleteNode(ev.Node.Name)
			if err != nil {
				log.Errorf("Error deleting node: %v", err)
				continue
			}
		}
	}
}

// Only run on the nodes
func watchSubnets(oc *OsdnController) {
	clusterEvent := make(chan *HostSubnetEvent)
	go oc.Registry.WatchSubnets(clusterEvent)
	for {
		ev := <-clusterEvent
		if ev.HostSubnet.HostIP == oc.localIP {
			continue
		}
		switch ev.Type {
		case Added:
			if err := oc.validateNode(ev.HostSubnet.HostIP); err != nil {
				log.Errorf("Ignoring invalid subnet for node %s: %v", ev.HostSubnet.HostIP, err)
				continue
			}
			oc.pluginHooks.AddHostSubnetRules(ev.HostSubnet)
		case Deleted:
			oc.pluginHooks.DeleteHostSubnetRules(ev.HostSubnet)
		}
	}
}

func (oc *OsdnController) validateNode(nodeIP string) error {
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
