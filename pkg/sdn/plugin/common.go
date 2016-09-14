package plugin

import (
	"fmt"
	"net"
	"strings"

	osclient "github.com/openshift/origin/pkg/client"
	osapi "github.com/openshift/origin/pkg/sdn/api"

	kapi "k8s.io/kubernetes/pkg/api"
)

func getPodContainerID(pod *kapi.Pod) string {
	if len(pod.Status.ContainerStatuses) > 0 {
		// Extract only container ID, pod.Status.ContainerStatuses[0].ContainerID is of the format: docker://<containerID>
		if parts := strings.Split(pod.Status.ContainerStatuses[0].ContainerID, "://"); len(parts) > 1 {
			return parts[1]
		}
	}
	return ""
}

func hostSubnetToString(subnet *osapi.HostSubnet) string {
	return fmt.Sprintf("%s (host: %q, ip: %q, subnet: %q)", subnet.Name, subnet.Host, subnet.HostIP, subnet.Subnet)
}

type NetworkInfo struct {
	ClusterNetwork   *net.IPNet
	ServiceNetwork   *net.IPNet
	HostSubnetLength uint32
	PluginName       string
}

func validateClusterNetwork(network string, hostSubnetLength uint32, serviceNetwork string, pluginName string) (*NetworkInfo, error) {
	_, cn, err := net.ParseCIDR(network)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse ClusterNetwork CIDR %s: %v", network, err)
	}

	_, sn, err := net.ParseCIDR(serviceNetwork)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse ServiceNetwork CIDR %s: %v", serviceNetwork, err)
	}

	if hostSubnetLength <= 0 || hostSubnetLength > 32 {
		return nil, fmt.Errorf("Invalid HostSubnetLength %d (not between 1 and 32)", hostSubnetLength)
	}

	return &NetworkInfo{
		ClusterNetwork:   cn,
		ServiceNetwork:   sn,
		HostSubnetLength: hostSubnetLength,
		PluginName:       pluginName,
	}, nil
}

func (ni *NetworkInfo) validateNodeIP(nodeIP string) error {
	if nodeIP == "" || nodeIP == "127.0.0.1" {
		return fmt.Errorf("Invalid node IP %q", nodeIP)
	}

	// Ensure each node's NodeIP is not contained by the cluster network,
	// which could cause a routing loop. (rhbz#1295486)
	ipaddr := net.ParseIP(nodeIP)
	if ipaddr == nil {
		return fmt.Errorf("Failed to parse node IP %s", nodeIP)
	}

	if ni.ClusterNetwork.Contains(ipaddr) {
		return fmt.Errorf("Node IP %s conflicts with cluster network %s", nodeIP, ni.ClusterNetwork.String())
	}

	return nil
}

func getNetworkInfo(osClient *osclient.Client) (*NetworkInfo, error) {
	cn, err := osClient.ClusterNetwork().Get(osapi.ClusterNetworkDefault)
	if err != nil {
		return nil, err
	}

	return validateClusterNetwork(cn.Network, cn.HostSubnetLength, cn.ServiceNetwork, cn.PluginName)
}
