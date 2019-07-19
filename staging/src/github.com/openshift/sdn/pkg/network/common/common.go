package common

import (
	"fmt"
	"net"

	kapi "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	networkv1 "github.com/openshift/api/network/v1"
	networkclient "github.com/openshift/client-go/network/clientset/versioned"
	"github.com/openshift/library-go/pkg/network/networkutils"
)

func HostSubnetToString(subnet *networkv1.HostSubnet) string {
	return fmt.Sprintf("%s (host: %q, ip: %q, subnet: %q)", subnet.Name, subnet.Host, subnet.HostIP, subnet.Subnet)
}

func ClusterNetworkToString(n *networkv1.ClusterNetwork) string {
	return fmt.Sprintf("%s (network: %q, hostSubnetBits: %d, serviceNetwork: %q, pluginName: %q)", n.Name, n.Network, n.HostSubnetLength, n.ServiceNetwork, n.PluginName)
}

func ClusterNetworkListContains(clusterNetworks []ParsedClusterNetworkEntry, ipaddr net.IP) (*net.IPNet, bool) {
	for _, cn := range clusterNetworks {
		if cn.ClusterCIDR.Contains(ipaddr) {
			return cn.ClusterCIDR, true
		}
	}
	return nil, false
}

type ParsedClusterNetwork struct {
	PluginName      string
	ClusterNetworks []ParsedClusterNetworkEntry
	ServiceNetwork  *net.IPNet
	VXLANPort       uint32
	MTU             uint32
}

type ParsedClusterNetworkEntry struct {
	ClusterCIDR      *net.IPNet
	HostSubnetLength uint32
}

func ParseClusterNetwork(cn *networkv1.ClusterNetwork) (*ParsedClusterNetwork, error) {
	pcn := &ParsedClusterNetwork{
		PluginName:      cn.PluginName,
		ClusterNetworks: make([]ParsedClusterNetworkEntry, 0, len(cn.ClusterNetworks)),
	}

	for _, entry := range cn.ClusterNetworks {
		cidr, err := networkutils.ParseCIDRMask(entry.CIDR)
		if err != nil {
			_, cidr, err = net.ParseCIDR(entry.CIDR)
			if err != nil {
				return nil, fmt.Errorf("failed to parse ClusterNetwork CIDR %s: %v", entry.CIDR, err)
			}
			utilruntime.HandleError(fmt.Errorf("Configured clusterNetworks value %q is invalid; treating it as %q", entry.CIDR, cidr.String()))
		}
		pcn.ClusterNetworks = append(pcn.ClusterNetworks, ParsedClusterNetworkEntry{ClusterCIDR: cidr, HostSubnetLength: entry.HostSubnetLength})
	}

	var err error
	pcn.ServiceNetwork, err = networkutils.ParseCIDRMask(cn.ServiceNetwork)
	if err != nil {
		_, pcn.ServiceNetwork, err = net.ParseCIDR(cn.ServiceNetwork)
		if err != nil {
			return nil, fmt.Errorf("failed to parse ServiceNetwork CIDR %s: %v", cn.ServiceNetwork, err)
		}
		utilruntime.HandleError(fmt.Errorf("Configured serviceNetworkCIDR value %q is invalid; treating it as %q", cn.ServiceNetwork, pcn.ServiceNetwork.String()))
	}

	if cn.VXLANPort != nil {
		pcn.VXLANPort = *cn.VXLANPort
	} else {
		pcn.VXLANPort = 4789
	}

	if cn.MTU != nil {
		pcn.MTU = *cn.MTU
	} else {
		pcn.MTU = 1450
	}

	return pcn, nil
}

func (pcn *ParsedClusterNetwork) ValidateNodeIP(nodeIP string) error {
	if nodeIP == "" || nodeIP == "127.0.0.1" {
		return fmt.Errorf("invalid node IP %q", nodeIP)
	}

	// Ensure each node's NodeIP is not contained by the cluster network,
	// which could cause a routing loop. (rhbz#1295486)
	ipaddr := net.ParseIP(nodeIP)
	if ipaddr == nil {
		return fmt.Errorf("failed to parse node IP %s", nodeIP)
	}

	if conflictingCIDR, found := ClusterNetworkListContains(pcn.ClusterNetworks, ipaddr); found {
		return fmt.Errorf("node IP %s conflicts with cluster network %s", nodeIP, conflictingCIDR.String())
	}
	if pcn.ServiceNetwork.Contains(ipaddr) {
		return fmt.Errorf("node IP %s conflicts with service network %s", nodeIP, pcn.ServiceNetwork.String())
	}

	return nil
}

func (pcn *ParsedClusterNetwork) CheckHostNetworks(hostIPNets []*net.IPNet) error {
	errList := []error{}
	for _, ipNet := range hostIPNets {
		for _, clusterNetwork := range pcn.ClusterNetworks {
			if CIDRsOverlap(ipNet.String(), clusterNetwork.ClusterCIDR.String()) {
				errList = append(errList, fmt.Errorf("cluster IP: %s conflicts with host network: %s", clusterNetwork.ClusterCIDR.IP.String(), ipNet.String()))
			}
		}
		if CIDRsOverlap(ipNet.String(), pcn.ServiceNetwork.String()) {
			errList = append(errList, fmt.Errorf("service IP: %s conflicts with host network: %s", pcn.ServiceNetwork.String(), ipNet.String()))
		}
	}
	return kerrors.NewAggregate(errList)
}

func (pcn *ParsedClusterNetwork) CheckClusterObjects(subnets []networkv1.HostSubnet, pods []kapi.Pod, services []kapi.Service) error {
	var errList []error

	for _, subnet := range subnets {
		subnetIP, _, _ := net.ParseCIDR(subnet.Subnet)
		if subnetIP == nil {
			errList = append(errList, fmt.Errorf("failed to parse network address: %s", subnet.Subnet))
		} else if _, contains := ClusterNetworkListContains(pcn.ClusterNetworks, subnetIP); !contains {
			errList = append(errList, fmt.Errorf("existing node subnet: %s is not part of any cluster network CIDR", subnet.Subnet))
		}
		if len(errList) >= 10 {
			break
		}
	}
	for _, pod := range pods {
		if pod.Spec.HostNetwork {
			continue
		}
		if _, contains := ClusterNetworkListContains(pcn.ClusterNetworks, net.ParseIP(pod.Status.PodIP)); !contains && pod.Status.PodIP != "" {
			errList = append(errList, fmt.Errorf("existing pod %s:%s with IP %s is not part of cluster network", pod.Namespace, pod.Name, pod.Status.PodIP))
			if len(errList) >= 10 {
				break
			}
		}
	}
	for _, svc := range services {
		svcIP := net.ParseIP(svc.Spec.ClusterIP)
		if svcIP != nil && !pcn.ServiceNetwork.Contains(svcIP) {
			errList = append(errList, fmt.Errorf("existing service %s:%s with IP %s is not part of service network %s", svc.Namespace, svc.Name, svc.Spec.ClusterIP, pcn.ServiceNetwork.String()))
			if len(errList) >= 10 {
				break
			}
		}
	}

	if len(errList) >= 10 {
		errList = append(errList, fmt.Errorf("too many errors... truncating"))
	}
	return kerrors.NewAggregate(errList)
}

func GetParsedClusterNetwork(networkClient networkclient.Interface) (*ParsedClusterNetwork, error) {
	cn, err := networkClient.NetworkV1().ClusterNetworks().Get(networkv1.ClusterNetworkDefault, v1.GetOptions{})
	if err != nil {
		return nil, err
	}
	if err = ValidateClusterNetwork(cn); err != nil {
		return nil, fmt.Errorf("ClusterNetwork is invalid (%v)", err)
	}
	return ParseClusterNetwork(cn)
}

// Generate the default gateway IP Address for a subnet
func GenerateDefaultGateway(sna *net.IPNet) net.IP {
	ip := sna.IP.To4()
	return net.IPv4(ip[0], ip[1], ip[2], ip[3]|0x1)
}

// Return Host IP Networks
// Ignores provided interfaces and filters loopback and non IPv4 addrs.
func GetHostIPNetworks(skipInterfaces []string) ([]*net.IPNet, []net.IP, error) {
	hostInterfaces, err := net.Interfaces()
	if err != nil {
		return nil, nil, err
	}

	skipInterfaceMap := make(map[string]bool)
	for _, ifaceName := range skipInterfaces {
		skipInterfaceMap[ifaceName] = true
	}

	errList := []error{}
	var hostIPNets []*net.IPNet
	var hostIPs []net.IP
	for _, iface := range hostInterfaces {
		if skipInterfaceMap[iface.Name] {
			continue
		}

		ifAddrs, err := iface.Addrs()
		if err != nil {
			errList = append(errList, err)
			continue
		}
		for _, addr := range ifAddrs {
			ip, ipNet, err := net.ParseCIDR(addr.String())
			if err != nil {
				errList = append(errList, err)
				continue
			}

			// Skip loopback and non IPv4 addrs
			if !ip.IsLoopback() && ip.To4() != nil {
				hostIPNets = append(hostIPNets, ipNet)
				hostIPs = append(hostIPs, ip)
			}
		}
	}
	return hostIPNets, hostIPs, kerrors.NewAggregate(errList)
}
