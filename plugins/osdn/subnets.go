package osdn

import (
	"fmt"
	"net"
	"time"

	log "github.com/golang/glog"

	"github.com/openshift/openshift-sdn/pkg/netutils"
	"github.com/openshift/openshift-sdn/plugins/osdn/api"
	osapi "github.com/openshift/origin/pkg/sdn/api"

	kapi "k8s.io/kubernetes/pkg/api"
)

func (oc *OsdnController) SubnetStartMaster(clusterNetwork *net.IPNet, hostSubnetLength uint) error {
	subrange := make([]string, 0)
	subnets, _, err := oc.Registry.GetSubnets()
	if err != nil {
		log.Errorf("Error in initializing/fetching subnets: %v", err)
		return err
	}
	for _, sub := range subnets {
		subrange = append(subrange, sub.Subnet)
		if err := oc.validateNode(sub.HostIP); err != nil {
			// Don't error out; just warn so the error can be corrected with 'oc'
			log.Errorf("Failed to validate HostSubnet %s: %v", err)
		}
	}

	oc.subnetAllocator, err = netutils.NewSubnetAllocator(clusterNetwork.String(), hostSubnetLength, subrange)
	if err != nil {
		return err
	}

	getNodes := func(registry *Registry) (interface{}, string, error) {
		return registry.GetNodes()
	}
	result, err := oc.watchAndGetResource("Node", watchNodes, getNodes)
	if err != nil {
		return err
	}

	// Make sure each node has a Subnet allocated
	nodes := result.([]kapi.Node)
	for _, node := range nodes {
		nodeIP, err := GetNodeIP(&node)
		if err != nil {
			// Don't error out; just warn so the error can be corrected by admin
			log.Errorf("Failed to get Node %s IP: %v", node.Name, err)
			continue
		}

		err = oc.validateNode(nodeIP)
		if err != nil {
			// Don't error out; just warn so the error can be corrected by admin
			log.Errorf("Failed to validate Node %s: %v", node.Name, err)
			continue
		}
		_, err = oc.Registry.GetSubnet(node.Name)
		if err == nil {
			// subnet already exists, continue
			continue
		}
		err = oc.addNode(node.Name, nodeIP)
		if err != nil {
			return err
		}
	}
	return nil
}

func (oc *OsdnController) addNode(nodeName string, nodeIP string) error {
	sn, err := oc.subnetAllocator.GetNetwork()
	if err != nil {
		log.Errorf("Error creating network for node %s.", nodeName)
		return err
	}

	if nodeIP == "" || nodeIP == "127.0.0.1" {
		return fmt.Errorf("Invalid node IP")
	}

	err = oc.Registry.CreateSubnet(nodeName, nodeIP, sn.String())
	if err != nil {
		log.Errorf("Error writing subnet to etcd for node %s: %v", nodeName, sn)
		return err
	}
	return nil
}

func (oc *OsdnController) deleteNode(nodeName string) error {
	sub, err := oc.Registry.GetSubnet(nodeName)
	if err != nil {
		log.Errorf("Error fetching subnet for node %s for delete operation.", nodeName)
		return err
	}
	_, ipnet, err := net.ParseCIDR(sub.Subnet)
	if err != nil {
		log.Errorf("Error parsing subnet for node %s for deletion: %s", nodeName, sub.Subnet)
		return err
	}
	oc.subnetAllocator.ReleaseNetwork(ipnet)
	return oc.Registry.DeleteSubnet(nodeName)
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

	getSubnets := func(registry *Registry) (interface{}, string, error) {
		return registry.GetSubnets()
	}
	result, err := oc.watchAndGetResource("HostSubnet", watchSubnets, getSubnets)
	if err != nil {
		return false, err
	}
	subnets := result.([]osapi.HostSubnet)
	for _, s := range subnets {
		oc.pluginHooks.AddOFRules(s.HostIP, s.Subnet, oc.localIP)
	}

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

	oc.localSubnet = subnet
	return nil
}

// Only run on the master
func watchNodes(oc *OsdnController, ready chan<- bool, start <-chan string) {
	stop := make(chan bool)
	nodeEvent := make(chan *api.NodeEvent)
	go oc.Registry.WatchNodes(nodeEvent, ready, start, stop)
	for {
		select {
		case ev := <-nodeEvent:
			switch ev.Type {
			case api.Added:
				nodeIP, nodeErr := GetNodeIP(ev.Node)
				if nodeErr == nil {
					nodeErr = oc.validateNode(nodeIP)
				}

				sub, err := oc.Registry.GetSubnet(ev.Node.Name)
				if err != nil {
					if nodeErr == nil {
						// subnet does not exist already
						oc.addNode(ev.Node.Name, nodeIP)
					} else {
						log.Errorf("Ignoring invalid node %s/%s: %v", ev.Node.Name, nodeIP, nodeErr)
					}
				} else {
					if sub.HostIP != nodeIP {
						err = oc.Registry.DeleteSubnet(ev.Node.Name)
						if err != nil {
							log.Errorf("Error deleting subnet for node %s, old ip %s", ev.Node.Name, sub.HostIP)
							continue
						}
						if nodeErr == nil {
							err = oc.Registry.CreateSubnet(ev.Node.Name, nodeIP, sub.Subnet)
							if err != nil {
								log.Errorf("Error creating subnet for node %s, ip %s", ev.Node.Name, sub.HostIP)
								continue
							}
						} else {
							log.Errorf("Ignoring creating invalid node %s/%s: %v", ev.Node.Name, nodeIP, nodeErr)
						}
					}
				}
			case api.Deleted:
				oc.deleteNode(ev.Node.Name)
			}
		case <-oc.sig:
			log.Error("Signal received. Stopping watching of nodes.")
			stop <- true
			return
		}
	}
}

// Only run on the nodes
func watchSubnets(oc *OsdnController, ready chan<- bool, start <-chan string) {
	stop := make(chan bool)
	clusterEvent := make(chan *api.HostSubnetEvent)
	go oc.Registry.WatchSubnets(clusterEvent, ready, start, stop)
	for {
		select {
		case ev := <-clusterEvent:
			switch ev.Type {
			case api.Added:
				if err := oc.validateNode(ev.HostSubnet.HostIP); err != nil {
					log.Errorf("Ignoring invalid subnet for node %s: %v", ev.HostSubnet.HostIP, err)
					continue
				}
				// add openflow rules
				oc.pluginHooks.AddOFRules(ev.HostSubnet.HostIP, ev.HostSubnet.Subnet, oc.localIP)
			case api.Deleted:
				// delete openflow rules meant for the node
				oc.pluginHooks.DelOFRules(ev.HostSubnet.HostIP, oc.localIP)
			}
		case <-oc.sig:
			stop <- true
			return
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
