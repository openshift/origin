package ovssubnet

import (
	"errors"
	"fmt"
	log "github.com/golang/glog"
	"net"
	"time"

	"github.com/openshift/openshift-sdn/ovssubnet/api"
	"github.com/openshift/openshift-sdn/ovssubnet/controller/kube"
	"github.com/openshift/openshift-sdn/ovssubnet/controller/lbr"
	"github.com/openshift/openshift-sdn/ovssubnet/controller/multitenant"
	"github.com/openshift/openshift-sdn/pkg/netutils"
)

const (
	// Maximum VXLAN Network Identifier as per RFC#7348
	MaxVNID = ((1 << 24) - 1)
)

type OvsController struct {
	subnetRegistry  api.SubnetRegistry
	localIP         string
	localSubnet     *api.Subnet
	hostName        string
	subnetAllocator *netutils.SubnetAllocator
	sig             chan struct{}
	ready           chan struct{}
	flowController  FlowController
	VNIDMap         map[string]uint
	netIDManager    *netutils.NetIDAllocator
}

type FlowController interface {
	Setup(localSubnetIP, globalSubnetIP, servicesSubnetIP string) error
	AddOFRules(nodeIP, localSubnetIP, localIP string) error
	DelOFRules(nodeIP, localIP string) error
	AddServiceOFRules(netID uint, IP string, protocol api.ServiceProtocol, port uint) error
	DelServiceOFRules(netID uint, IP string, protocol api.ServiceProtocol, port uint) error
}

func NewKubeController(sub api.SubnetRegistry, hostname string, selfIP string, ready chan struct{}) (*OvsController, error) {
	kubeController, err := NewController(sub, hostname, selfIP, ready)
	if err == nil {
		kubeController.flowController = kube.NewFlowController()
	}
	return kubeController, err
}

func NewMultitenantController(sub api.SubnetRegistry, hostname string, selfIP string, ready chan struct{}) (*OvsController, error) {
	mtController, err := NewController(sub, hostname, selfIP, ready)
	if err == nil {
		mtController.flowController = multitenant.NewFlowController()
	}
	return mtController, err
}

func NewDefaultController(sub api.SubnetRegistry, hostname string, selfIP string, ready chan struct{}) (*OvsController, error) {
	defaultController, err := NewController(sub, hostname, selfIP, ready)
	if err == nil {
		defaultController.flowController = lbr.NewFlowController()
	}
	return defaultController, err
}

func NewController(sub api.SubnetRegistry, hostname string, selfIP string, ready chan struct{}) (*OvsController, error) {
	if selfIP == "" {
		addrs, err := net.LookupIP(hostname)
		if err != nil {
			log.Errorf("Failed to lookup IP Address for %s", hostname)
			return nil, err
		}
		for _, addr := range addrs {
			if addr.String() != "127.0.0.1" {
				selfIP = addr.String()
				break
			}
		}
		if selfIP == "" {
			return nil, fmt.Errorf("failed to lookup valid IP Address for %s (%v)", hostname, addrs)
		}
	}
	log.Infof("Self IP: %s.", selfIP)
	return &OvsController{
		subnetRegistry:  sub,
		localIP:         selfIP,
		hostName:        hostname,
		localSubnet:     nil,
		subnetAllocator: nil,
		VNIDMap:         make(map[string]uint),
		sig:             make(chan struct{}),
		ready:           ready,
	}, nil
}

func (oc *OvsController) StartMaster(sync bool, containerNetwork string, containerSubnetLength uint, serviceNetwork string) error {
	// wait a minute for etcd to come alive
	status := oc.subnetRegistry.CheckEtcdIsAlive(60)
	if !status {
		log.Errorf("Etcd not running?")
		return errors.New("Etcd not reachable. Sync cluster check failed.")
	}
	// initialize the node key
	if sync {
		err := oc.subnetRegistry.InitNodes()
		if err != nil {
			log.Infof("Node path already initialized.")
		}
	}

	// initialize the subnet key?
	oc.subnetRegistry.InitSubnets()
	subrange := make([]string, 0)
	subnets, err := oc.subnetRegistry.GetSubnets()
	if err != nil {
		log.Errorf("Error in initializing/fetching subnets: %v", err)
		return err
	}
	for _, sub := range *subnets {
		subrange = append(subrange, sub.SubnetIP)
	}

	err = oc.subnetRegistry.WriteNetworkConfig(containerNetwork, containerSubnetLength, serviceNetwork)
	if err != nil {
		return err
	}

	oc.subnetAllocator, err = netutils.NewSubnetAllocator(containerNetwork, containerSubnetLength, subrange)
	if err != nil {
		return err
	}
	err = oc.ServeExistingNodes()
	if err != nil {
		log.Warningf("Error initializing existing nodes: %v", err)
		// no worry, we can still keep watching it.
	}
	if _, is_mt := oc.flowController.(*multitenant.FlowController); is_mt {
		nets, err := oc.subnetRegistry.GetNetNamespaces()
		if err != nil {
			return err
		}
		inUse := make([]uint, 0)
		for _, net := range nets {
			inUse = append(inUse, net.NetID)
			oc.VNIDMap[net.Name] = net.NetID
		}
		// VNID: 0 reserved for default namespace and can reach any network in the cluster
		// VNID: 1 to 9 are internally reserved for any special cases in the future
		oc.netIDManager, err = netutils.NewNetIDAllocator(10, MaxVNID, inUse)
		if err != nil {
			return err
		}
		go oc.watchNetworks()
	}
	go oc.watchNodes()
	return nil
}

func (oc *OvsController) watchNetworks() {
	nsevent := make(chan *api.NamespaceEvent)
	stop := make(chan bool)
	go oc.subnetRegistry.WatchNamespaces(nsevent, stop)
	for {
		select {
		case ev := <-nsevent:
			switch ev.Type {
			case api.Added:
				_, err := oc.subnetRegistry.GetNetNamespace(ev.Name)
				if err != nil {
					netid, err := oc.netIDManager.GetNetID()
					if err != nil {
						log.Error("Error getting new network IDS: %v", err)
						continue
					}
					err = oc.subnetRegistry.WriteNetNamespace(ev.Name, netid)
					if err != nil {
						log.Error("Error writing new network ID: %v", err)
						continue
					}
					oc.VNIDMap[ev.Name] = netid
				}
			case api.Deleted:
				err := oc.subnetRegistry.DeleteNetNamespace(ev.Name)
				if err != nil {
					log.Error("Error while deleting Net Id: %v", err)
					continue
				}
				netid, ok := oc.VNIDMap[ev.Name]
				if !ok {
					log.Error("Error while fetching Net Id for namespace: %s", ev.Name)
					continue
				}
				err = oc.netIDManager.ReleaseNetID(netid)
				if err != nil {
					log.Error("Error while releasing Net Id: %v", err)
					continue
				}
				delete(oc.VNIDMap, ev.Name)
			}
		case <-oc.sig:
			log.Error("Signal received. Stopping watching of nodes.")
			stop <- true
			return
		}
	}
}

func (oc *OvsController) ServeExistingNodes() error {
	nodes, err := oc.subnetRegistry.GetNodes()
	if err != nil {
		return err
	}

	for _, nodeName := range *nodes {
		_, err := oc.subnetRegistry.GetSubnet(nodeName)
		if err == nil {
			// subnet already exists, continue
			continue
		}
		err = oc.AddNode(nodeName, "")
		if err != nil {
			return err
		}
	}
	return nil
}

func (oc *OvsController) getNodeIP(nodeName string) (string, error) {
	ip := net.ParseIP(nodeName)
	if ip == nil {
		addrs, err := net.LookupIP(nodeName)
		if err != nil {
			log.Errorf("Failed to lookup IP address for node %s: %v", nodeName, err)
			return "", err
		}
		for _, addr := range addrs {
			if addr.String() != "127.0.0.1" {
				ip = addr
				break
			}
		}
	}
	if ip == nil || len(ip.String()) == 0 {
		return "", fmt.Errorf("Failed to obtain IP address from node label: %s", nodeName)
	}
	return ip.String(), nil
}

func (oc *OvsController) AddNode(nodeName string, nodeIP string) error {
	sn, err := oc.subnetAllocator.GetNetwork()
	if err != nil {
		log.Errorf("Error creating network for node %s.", nodeName)
		return err
	}

	if nodeIP == "" || nodeIP == "127.0.0.1" {
		nodeIP, err = oc.getNodeIP(nodeName)
		if err != nil {
			return err
		}
	}

	subnet := &api.Subnet{
		NodeIP:   nodeIP,
		SubnetIP: sn.String(),
	}
	err = oc.subnetRegistry.CreateSubnet(nodeName, subnet)
	if err != nil {
		log.Errorf("Error writing subnet to etcd for node %s: %v", nodeName, sn)
		return err
	}
	return nil
}

func (oc *OvsController) DeleteNode(nodeName string) error {
	sub, err := oc.subnetRegistry.GetSubnet(nodeName)
	if err != nil {
		log.Errorf("Error fetching subnet for node %s for delete operation.", nodeName)
		return err
	}
	_, ipnet, err := net.ParseCIDR(sub.SubnetIP)
	if err != nil {
		log.Errorf("Error parsing subnet for node %s for deletion: %s", nodeName, sub.SubnetIP)
		return err
	}
	oc.subnetAllocator.ReleaseNetwork(ipnet)
	return oc.subnetRegistry.DeleteSubnet(nodeName)
}

func (oc *OvsController) syncWithMaster() error {
	return oc.subnetRegistry.CreateNode(oc.hostName, oc.localIP)
}

func (oc *OvsController) StartNode(sync, skipsetup bool) error {
	if sync {
		err := oc.syncWithMaster()
		if err != nil {
			log.Errorf("Failed to register with master: %v", err)
			return err
		}
	}
	err := oc.initSelfSubnet()
	if err != nil {
		log.Errorf("Failed to get subnet for this host: %v", err)
		return err
	}

	// call flow controller's setup
	if !skipsetup {
		// Assume we are working with IPv4
		containerNetwork, err := oc.subnetRegistry.GetContainerNetwork()
		if err != nil {
			log.Errorf("Failed to obtain ContainerNetwork: %v", err)
			return err
		}
		servicesNetwork, err := oc.subnetRegistry.GetServicesNetwork()
		if err != nil {
			log.Errorf("Failed to obtain ServicesNetwork: %v", err)
			return err
		}
		err = oc.flowController.Setup(oc.localSubnet.SubnetIP, containerNetwork, servicesNetwork)
		if err != nil {
			return err
		}
	}
	subnets, err := oc.subnetRegistry.GetSubnets()
	if err != nil {
		log.Errorf("Could not fetch existing subnets: %v", err)
	}
	for _, s := range *subnets {
		oc.flowController.AddOFRules(s.NodeIP, s.SubnetIP, oc.localIP)
	}
	if _, ok := oc.flowController.(*multitenant.FlowController); ok {
		nslist, err := oc.subnetRegistry.GetNetNamespaces()
		if err != nil {
			return err
		}
		for _, ns := range nslist {
			oc.VNIDMap[ns.Name] = ns.NetID
		}
		go oc.watchVnids()

		services, err := oc.subnetRegistry.GetServices()
		if err != nil {
			return err
		}
		for _, svc := range *services {
			oc.flowController.AddServiceOFRules(oc.VNIDMap[svc.Namespace], svc.IP, svc.Protocol, svc.Port)
		}
		go oc.watchServices()
	}
	go oc.watchCluster()

	if oc.ready != nil {
		close(oc.ready)
	}

	return err
}

func (oc *OvsController) watchVnids() {
	netNsEvent := make(chan *api.NetNamespaceEvent)
	stop := make(chan bool)
	go oc.subnetRegistry.WatchNetNamespaces(netNsEvent, stop)
	for {
		select {
		case ev := <-netNsEvent:
			switch ev.Type {
			case api.Added:
				oc.VNIDMap[ev.Name] = ev.NetID
			case api.Deleted:
				delete(oc.VNIDMap, ev.Name)
			}
		case <-oc.sig:
			log.Error("Signal received. Stopping watching of NetNamespaces.")
			stop <- true
			return
		}
	}
}

func (oc *OvsController) initSelfSubnet() error {
	// get subnet for self
	for {
		sub, err := oc.subnetRegistry.GetSubnet(oc.hostName)
		if err != nil {
			log.Errorf("Could not find an allocated subnet for node %s: %s. Waiting...", oc.hostName, err)
			time.Sleep(2 * time.Second)
			continue
		}
		oc.localSubnet = sub
		return nil
	}
}

func (oc *OvsController) watchNodes() {
	// watch latest?
	stop := make(chan bool)
	nodeEvent := make(chan *api.NodeEvent)
	go oc.subnetRegistry.WatchNodes(nodeEvent, stop)
	for {
		select {
		case ev := <-nodeEvent:
			switch ev.Type {
			case api.Added:
				sub, err := oc.subnetRegistry.GetSubnet(ev.NodeName)
				if err != nil {
					// subnet does not exist already
					oc.AddNode(ev.NodeName, ev.NodeIP)
				} else {
					// Current node IP is obtained from event, ev.NodeIP to
					// avoid cached/stale IP lookup by net.LookupIP()
					if sub.NodeIP != ev.NodeIP {
						err = oc.subnetRegistry.DeleteSubnet(ev.NodeName)
						if err != nil {
							log.Errorf("Error deleting subnet for node %s, old ip %s", ev.NodeName, sub.NodeIP)
							continue
						}
						sub.NodeIP = ev.NodeIP
						err = oc.subnetRegistry.CreateSubnet(ev.NodeName, sub)
						if err != nil {
							log.Errorf("Error creating subnet for node %s, ip %s", ev.NodeName, sub.NodeIP)
							continue
						}
					}
				}
			case api.Deleted:
				oc.DeleteNode(ev.NodeName)
			}
		case <-oc.sig:
			log.Error("Signal received. Stopping watching of nodes.")
			stop <- true
			return
		}
	}
}

func (oc *OvsController) watchServices() {
	stop := make(chan bool)
	svcevent := make(chan *api.ServiceEvent)
	go oc.subnetRegistry.WatchServices(svcevent, stop)
	for {
		select {
		case ev := <-svcevent:
			netid := oc.VNIDMap[ev.Service.Namespace]
			switch ev.Type {
			case api.Added:
				oc.flowController.AddServiceOFRules(netid, ev.Service.IP, ev.Service.Protocol, ev.Service.Port)
			case api.Deleted:
				oc.flowController.DelServiceOFRules(netid, ev.Service.IP, ev.Service.Protocol, ev.Service.Port)
			}
		case <-oc.sig:
			log.Error("Signal received. Stopping watching of services.")
			stop <- true
			return
		}
	}
}

func (oc *OvsController) watchCluster() {
	stop := make(chan bool)
	clusterEvent := make(chan *api.SubnetEvent)
	go oc.subnetRegistry.WatchSubnets(clusterEvent, stop)
	for {
		select {
		case ev := <-clusterEvent:
			switch ev.Type {
			case api.Added:
				// add openflow rules
				oc.flowController.AddOFRules(ev.Subnet.NodeIP, ev.Subnet.SubnetIP, oc.localIP)
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

func (oc *OvsController) Stop() {
	close(oc.sig)
	//oc.sig <- struct{}{}
}
