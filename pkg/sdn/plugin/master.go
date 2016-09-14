package plugin

import (
	"fmt"
	"net"

	log "github.com/golang/glog"

	osclient "github.com/openshift/origin/pkg/client"
	osconfigapi "github.com/openshift/origin/pkg/cmd/server/api"
	osapi "github.com/openshift/origin/pkg/sdn/api"
	"github.com/openshift/origin/pkg/util/netutils"

	kapi "k8s.io/kubernetes/pkg/api"
	kapiunversioned "k8s.io/kubernetes/pkg/api/unversioned"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	kerrors "k8s.io/kubernetes/pkg/util/errors"
)

type OsdnMaster struct {
	registry        *Registry
	networkInfo     *NetworkInfo
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
	var err error
	master.networkInfo, err = validateClusterNetwork(networkConfig.ClusterNetworkCIDR, networkConfig.HostSubnetLength, networkConfig.ServiceNetworkCIDR, networkConfig.NetworkPluginName)
	if err != nil {
		return err
	}

	createConfig := false
	updateConfig := false
	cn, err := master.registry.oClient.ClusterNetwork().Get(osapi.ClusterNetworkDefault)
	if err == nil {
		if master.networkInfo.ClusterNetwork.String() != cn.Network ||
			master.networkInfo.HostSubnetLength != cn.HostSubnetLength ||
			master.networkInfo.ServiceNetwork.String() != cn.ServiceNetwork ||
			master.networkInfo.PluginName != cn.PluginName {
			updateConfig = true
		}
	} else {
		cn = &osapi.ClusterNetwork{
			TypeMeta:   kapiunversioned.TypeMeta{Kind: "ClusterNetwork"},
			ObjectMeta: kapi.ObjectMeta{Name: osapi.ClusterNetworkDefault},
		}
		createConfig = true
	}
	if createConfig || updateConfig {
		if err = master.validateNetworkConfig(); err != nil {
			return err
		}
		cn.Network = master.networkInfo.ClusterNetwork.String()
		cn.HostSubnetLength = master.networkInfo.HostSubnetLength
		cn.ServiceNetwork = master.networkInfo.ServiceNetwork.String()
		cn.PluginName = master.networkInfo.PluginName
	}

	if createConfig {
		cn, err := master.registry.oClient.ClusterNetwork().Create(cn)
		if err != nil {
			return err
		}
		log.Infof("Created ClusterNetwork %s", clusterNetworkToString(cn))
	} else if updateConfig {
		cn, err := master.registry.oClient.ClusterNetwork().Update(cn)
		if err != nil {
			return err
		}
		log.Infof("Updated ClusterNetwork %s", clusterNetworkToString(cn))
	}

	if err = master.SubnetStartMaster(master.networkInfo.ClusterNetwork, networkConfig.HostSubnetLength); err != nil {
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

func (master *OsdnMaster) validateNetworkConfig() error {
	hostIPNets, err := netutils.GetHostIPNetworks([]string{TUN, LBR})
	if err != nil {
		return err
	}

	ni := master.networkInfo
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
