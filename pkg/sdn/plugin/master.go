package plugin

import (
	"fmt"
	"net"

	log "github.com/golang/glog"

	osclient "github.com/openshift/origin/pkg/client"
	osconfigapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/util/netutils"

	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	kerrors "k8s.io/kubernetes/pkg/util/errors"
)

type OsdnMaster struct {
	registry        *Registry
	subnetAllocator *netutils.SubnetAllocator
	vnids           *masterVNIDMap
}

func StartMaster(networkConfig osconfigapi.MasterNetworkConfig, osClient *osclient.Client, kClient *kclient.Client) error {
	if !IsOpenShiftNetworkPlugin(networkConfig.NetworkPluginName) {
		return nil
	}

	log.Infof("Initializing SDN master of type %q", networkConfig.NetworkPluginName)

	master := &OsdnMaster{
		registry: newRegistry(osClient, kClient),
	}

	// Validate command-line/config parameters
	ni, err := validateClusterNetwork(networkConfig.ClusterNetworkCIDR, networkConfig.HostSubnetLength, networkConfig.ServiceNetworkCIDR, networkConfig.NetworkPluginName)
	if err != nil {
		return err
	}

	changed, net_err := master.isClusterNetworkChanged(ni)
	if changed {
		if err = master.validateNetworkConfig(ni); err != nil {
			return err
		}
		if err = master.registry.UpdateClusterNetwork(ni); err != nil {
			return err
		}
	} else if net_err != nil {
		if err = master.registry.CreateClusterNetwork(ni); err != nil {
			return err
		}
	}

	if err = master.SubnetStartMaster(ni.ClusterNetwork, networkConfig.HostSubnetLength); err != nil {
		return err
	}

	if IsOpenShiftMultitenantNetworkPlugin(networkConfig.NetworkPluginName) {
		master.vnids = newMasterVNIDMap()

		if err = master.VnidStartMaster(); err != nil {
			return err
		}
	}

	return nil
}

func (master *OsdnMaster) validateNetworkConfig(ni *NetworkInfo) error {
	hostIPNets, err := netutils.GetHostIPNetworks([]string{TUN, LBR})
	if err != nil {
		return err
	}

	errList := []error{}

	// Ensure cluster and service network don't overlap with host networks
	for _, ipNet := range hostIPNets {
		if ipNet.Contains(ni.ClusterNetwork.IP) {
			errList = append(errList, fmt.Errorf("Error: Cluster IP: %s conflicts with host network: %s", ni.ClusterNetwork.IP.String(), ipNet.String()))
		}
		if ni.ClusterNetwork.Contains(ipNet.IP) {
			errList = append(errList, fmt.Errorf("Error: Host network with IP: %s conflicts with cluster network: %s", ipNet.IP.String(), ni.ClusterNetwork.String()))
		}
		if ipNet.Contains(ni.ServiceNetwork.IP) {
			errList = append(errList, fmt.Errorf("Error: Service IP: %s conflicts with host network: %s", ni.ServiceNetwork.String(), ipNet.String()))
		}
		if ni.ServiceNetwork.Contains(ipNet.IP) {
			errList = append(errList, fmt.Errorf("Error: Host network with IP: %s conflicts with service network: %s", ipNet.IP.String(), ni.ServiceNetwork.String()))
		}
	}

	// Ensure each host subnet is within the cluster network
	subnets, err := master.registry.GetSubnets()
	if err != nil {
		return fmt.Errorf("Error in initializing/fetching subnets: %v", err)
	}
	for _, sub := range subnets {
		subnetIP, _, _ := net.ParseCIDR(sub.Subnet)
		if subnetIP == nil {
			errList = append(errList, fmt.Errorf("Failed to parse network address: %s", sub.Subnet))
			continue
		}
		if !ni.ClusterNetwork.Contains(subnetIP) {
			errList = append(errList, fmt.Errorf("Error: Existing node subnet: %s is not part of cluster network: %s", sub.Subnet, ni.ClusterNetwork.String()))
		}
	}

	// Ensure each service is within the services network
	services, err := master.registry.GetServices()
	if err != nil {
		return err
	}
	for _, svc := range services {
		if !ni.ServiceNetwork.Contains(net.ParseIP(svc.Spec.ClusterIP)) {
			errList = append(errList, fmt.Errorf("Error: Existing service with IP: %s is not part of service network: %s", svc.Spec.ClusterIP, ni.ServiceNetwork.String()))
		}
	}

	return kerrors.NewAggregate(errList)
}

func (master *OsdnMaster) isClusterNetworkChanged(curNetwork *NetworkInfo) (bool, error) {
	oldNetwork, err := master.registry.GetNetworkInfo()
	if err != nil {
		return false, err
	}

	if curNetwork.ClusterNetwork.String() != oldNetwork.ClusterNetwork.String() ||
		curNetwork.HostSubnetLength != oldNetwork.HostSubnetLength ||
		curNetwork.ServiceNetwork.String() != oldNetwork.ServiceNetwork.String() ||
		curNetwork.PluginName != oldNetwork.PluginName {
		return true, nil
	}
	return false, nil
}
