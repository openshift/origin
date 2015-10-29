package osdn

import (
	"fmt"
	"net"
	"time"

	log "github.com/golang/glog"

	"github.com/openshift/openshift-sdn/pkg/netutils"
	"github.com/openshift/openshift-sdn/plugins/osdn/api"
)

func (oc *OvsController) SubnetStartMaster(clusterNetworkCIDR string, clusterBitsPerSubnet uint, serviceNetworkCIDR string) error {
	subrange := make([]string, 0)
	subnets, _, err := oc.Registry.GetSubnets()
	if err != nil {
		log.Errorf("Error in initializing/fetching subnets: %v", err)
		return err
	}
	for _, sub := range subnets {
		subrange = append(subrange, sub.SubnetCIDR)
	}

	oc.subnetAllocator, err = netutils.NewSubnetAllocator(clusterNetworkCIDR, clusterBitsPerSubnet, subrange)
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
	nodes := result.([]api.Node)
	err = oc.serveExistingNodes(nodes)
	if err != nil {
		return err
	}

	return nil
}

func (oc *OvsController) serveExistingNodes(nodes []api.Node) error {
	for _, node := range nodes {
		_, err := oc.Registry.GetSubnet(node.Name)
		if err == nil {
			// subnet already exists, continue
			continue
		}
		err = oc.addNode(node.Name, node.IP)
		if err != nil {
			return err
		}
	}
	return nil
}

func (oc *OvsController) addNode(nodeName string, nodeIP string) error {
	sn, err := oc.subnetAllocator.GetNetwork()
	if err != nil {
		log.Errorf("Error creating network for node %s.", nodeName)
		return err
	}

	if nodeIP == "" || nodeIP == "127.0.0.1" {
		return fmt.Errorf("Invalid node IP")
	}

	subnet := &api.Subnet{
		NodeIP:     nodeIP,
		SubnetCIDR: sn.String(),
	}
	err = oc.Registry.CreateSubnet(nodeName, subnet)
	if err != nil {
		log.Errorf("Error writing subnet to etcd for node %s: %v", nodeName, sn)
		return err
	}
	return nil
}

func (oc *OvsController) deleteNode(nodeName string) error {
	sub, err := oc.Registry.GetSubnet(nodeName)
	if err != nil {
		log.Errorf("Error fetching subnet for node %s for delete operation.", nodeName)
		return err
	}
	_, ipnet, err := net.ParseCIDR(sub.SubnetCIDR)
	if err != nil {
		log.Errorf("Error parsing subnet for node %s for deletion: %s", nodeName, sub.SubnetCIDR)
		return err
	}
	oc.subnetAllocator.ReleaseNetwork(ipnet)
	return oc.Registry.DeleteSubnet(nodeName)
}

func (oc *OvsController) SubnetStartNode(mtu uint) error {
	err := oc.initSelfSubnet()
	if err != nil {
		return err
	}

	// Assume we are working with IPv4
	clusterNetworkCIDR, err := oc.Registry.GetClusterNetworkCIDR()
	if err != nil {
		log.Errorf("Failed to obtain ClusterNetwork: %v", err)
		return err
	}
	servicesNetworkCIDR, err := oc.Registry.GetServicesNetworkCIDR()
	if err != nil {
		log.Errorf("Failed to obtain ServicesNetwork: %v", err)
		return err
	}
	err = oc.flowController.Setup(oc.localSubnet.SubnetCIDR, clusterNetworkCIDR, servicesNetworkCIDR, mtu)
	if err != nil {
		return err
	}

	getSubnets := func(registry *Registry) (interface{}, string, error) {
		return registry.GetSubnets()
	}
	result, err := oc.watchAndGetResource("HostSubnet", watchSubnets, getSubnets)
	if err != nil {
		return err
	}
	subnets := result.([]api.Subnet)
	for _, s := range subnets {
		oc.flowController.AddOFRules(s.NodeIP, s.SubnetCIDR, oc.localIP)
	}

	return nil
}

func (oc *OvsController) initSelfSubnet() error {
	// timeout: 10 secs
	retries := 20
	retryInterval := 500 * time.Millisecond

	var err error
	var subnet *api.Subnet
	// Try every retryInterval and bail-out if it exceeds max retries
	for i := 0; i < retries; i++ {
		// Get subnet for current node
		subnet, err = oc.Registry.GetSubnet(oc.hostName)
		if err == nil {
			break
		}
		log.Warningf("Could not find an allocated subnet for node: %s, Waiting...", oc.hostName)
		time.Sleep(retryInterval)
	}
	if err != nil {
		return fmt.Errorf("Failed to get subnet for this host: %s, error: %v", oc.hostName, err)
	}
	oc.localSubnet = subnet
	return nil
}

func watchNodes(oc *OvsController, ready chan<- bool, start <-chan string) {
	stop := make(chan bool)
	nodeEvent := make(chan *api.NodeEvent)
	go oc.Registry.WatchNodes(nodeEvent, ready, start, stop)
	for {
		select {
		case ev := <-nodeEvent:
			switch ev.Type {
			case api.Added:
				sub, err := oc.Registry.GetSubnet(ev.Node.Name)
				if err != nil {
					// subnet does not exist already
					oc.addNode(ev.Node.Name, ev.Node.IP)
				} else {
					// Current node IP is obtained from event, ev.NodeIP to
					// avoid cached/stale IP lookup by net.LookupIP()
					if sub.NodeIP != ev.Node.IP {
						err = oc.Registry.DeleteSubnet(ev.Node.Name)
						if err != nil {
							log.Errorf("Error deleting subnet for node %s, old ip %s", ev.Node.Name, sub.NodeIP)
							continue
						}
						sub.NodeIP = ev.Node.IP
						err = oc.Registry.CreateSubnet(ev.Node.Name, sub)
						if err != nil {
							log.Errorf("Error creating subnet for node %s, ip %s", ev.Node.Name, sub.NodeIP)
							continue
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

func watchSubnets(oc *OvsController, ready chan<- bool, start <-chan string) {
	stop := make(chan bool)
	clusterEvent := make(chan *api.SubnetEvent)
	go oc.Registry.WatchSubnets(clusterEvent, ready, start, stop)
	for {
		select {
		case ev := <-clusterEvent:
			switch ev.Type {
			case api.Added:
				// add openflow rules
				oc.flowController.AddOFRules(ev.Subnet.NodeIP, ev.Subnet.SubnetCIDR, oc.localIP)
			case api.Deleted:
				// delete openflow rules meant for the node
				oc.flowController.DelOFRules(ev.Subnet.NodeIP, oc.localIP)
			}
		case <-oc.sig:
			stop <- true
			return
		}
	}
}
