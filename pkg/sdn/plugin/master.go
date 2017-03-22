package plugin

import (
	log "github.com/golang/glog"

	osclient "github.com/openshift/origin/pkg/client"
	osconfigapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/controller/shared"
	osapi "github.com/openshift/origin/pkg/sdn/api"
	"github.com/openshift/origin/pkg/util/netutils"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ktypes "k8s.io/apimachinery/pkg/types"
	kapi "k8s.io/kubernetes/pkg/api"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
)

type OsdnMaster struct {
	kClient         kclientset.Interface
	osClient        *osclient.Client
	networkInfo     *NetworkInfo
	subnetAllocator *netutils.SubnetAllocator
	vnids           *masterVNIDMap
	informers       shared.InformerFactory

	// Holds Node IP used in creating host subnet for a node
	hostSubnetNodeIPs map[ktypes.UID]string
}

func StartMaster(networkConfig osconfigapi.MasterNetworkConfig, osClient *osclient.Client, kClient kclientset.Interface, informers shared.InformerFactory) error {
	if !osapi.IsOpenShiftNetworkPlugin(networkConfig.NetworkPluginName) {
		return nil
	}

	log.Infof("Initializing SDN master of type %q", networkConfig.NetworkPluginName)

	master := &OsdnMaster{
		kClient:           kClient,
		osClient:          osClient,
		informers:         informers,
		hostSubnetNodeIPs: map[ktypes.UID]string{},
	}

	var err error
	master.networkInfo, err = parseNetworkInfo(networkConfig.ClusterNetworkCIDR, networkConfig.ServiceNetworkCIDR)
	if err != nil {
		return err
	}

	createConfig := false
	updateConfig := false
	cn, err := master.osClient.ClusterNetwork().Get(osapi.ClusterNetworkDefault, metav1.GetOptions{})
	if err == nil {
		if master.networkInfo.ClusterNetwork.String() != cn.Network ||
			networkConfig.HostSubnetLength != cn.HostSubnetLength ||
			master.networkInfo.ServiceNetwork.String() != cn.ServiceNetwork ||
			networkConfig.NetworkPluginName != cn.PluginName {
			updateConfig = true
		}
	} else {
		cn = &osapi.ClusterNetwork{
			TypeMeta:   metav1.TypeMeta{Kind: "ClusterNetwork"},
			ObjectMeta: metav1.ObjectMeta{Name: osapi.ClusterNetworkDefault},
		}
		createConfig = true
	}
	if createConfig || updateConfig {
		if err = master.checkClusterNetworkAgainstLocalNetworks(); err != nil {
			return err
		}
		if err = master.checkClusterNetworkAgainstClusterObjects(); err != nil {
			return err
		}
		cn.Network = master.networkInfo.ClusterNetwork.String()
		cn.HostSubnetLength = networkConfig.HostSubnetLength
		cn.ServiceNetwork = master.networkInfo.ServiceNetwork.String()
		cn.PluginName = networkConfig.NetworkPluginName
	}

	if createConfig {
		cn, err := master.osClient.ClusterNetwork().Create(cn)
		if err != nil {
			return err
		}
		log.Infof("Created ClusterNetwork %s", clusterNetworkToString(cn))
	} else if updateConfig {
		cn, err := master.osClient.ClusterNetwork().Update(cn)
		if err != nil {
			return err
		}
		log.Infof("Updated ClusterNetwork %s", clusterNetworkToString(cn))
	}

	if err = master.SubnetStartMaster(master.networkInfo.ClusterNetwork, networkConfig.HostSubnetLength); err != nil {
		return err
	}

	switch networkConfig.NetworkPluginName {
	case osapi.MultiTenantPluginName:
		master.vnids = newMasterVNIDMap(true)
		if err = master.VnidStartMaster(); err != nil {
			return err
		}
	case osapi.NetworkPolicyPluginName:
		master.vnids = newMasterVNIDMap(false)
		if err = master.VnidStartMaster(); err != nil {
			return err
		}
	}

	return nil
}

func (master *OsdnMaster) checkClusterNetworkAgainstLocalNetworks() error {
	hostIPNets, _, err := netutils.GetHostIPNetworks([]string{TUN})
	if err != nil {
		return err
	}
	return master.networkInfo.checkHostNetworks(hostIPNets)
}

func (master *OsdnMaster) checkClusterNetworkAgainstClusterObjects() error {
	var subnets []osapi.HostSubnet
	var pods []kapi.Pod
	var services []kapi.Service
	if subnetList, err := master.osClient.HostSubnets().List(metav1.ListOptions{}); err == nil {
		subnets = subnetList.Items
	}
	if podList, err := master.kClient.Core().Pods(metav1.NamespaceAll).List(metav1.ListOptions{}); err == nil {
		pods = podList.Items
	}
	if serviceList, err := master.kClient.Core().Services(metav1.NamespaceAll).List(metav1.ListOptions{}); err == nil {
		services = serviceList.Items
	}

	return master.networkInfo.checkClusterObjects(subnets, pods, services)
}
