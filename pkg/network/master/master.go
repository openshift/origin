package master

import (
	"fmt"
	"time"

	"github.com/golang/glog"

	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ktypes "k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	kinternalinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/internalversion"

	osconfigapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	"github.com/openshift/origin/pkg/network"
	networkapi "github.com/openshift/origin/pkg/network/apis/network"
	osapivalidation "github.com/openshift/origin/pkg/network/apis/network/validation"
	"github.com/openshift/origin/pkg/network/common"
	networkinformers "github.com/openshift/origin/pkg/network/generated/informers/internalversion"
	networkclient "github.com/openshift/origin/pkg/network/generated/internalclientset"
	"github.com/openshift/origin/pkg/util/netutils"
)

const (
	tun0 = "tun0"
)

type OsdnMaster struct {
	kClient             kclientset.Interface
	networkClient       networkclient.Interface
	networkInfo         *common.NetworkInfo
	subnetAllocatorList []*SubnetAllocator
	vnids               *masterVNIDMap

	kubeInformers    kinternalinformers.SharedInformerFactory
	networkInformers networkinformers.SharedInformerFactory

	// Holds Node IP used in creating host subnet for a node
	hostSubnetNodeIPs map[ktypes.UID]string
}

func Start(networkConfig osconfigapi.MasterNetworkConfig, networkClient networkclient.Interface,
	kClient kclientset.Interface, kubeInformers kinternalinformers.SharedInformerFactory,
	networkInformers networkinformers.SharedInformerFactory) error {
	glog.Infof("Initializing SDN master of type %q", networkConfig.NetworkPluginName)

	master := &OsdnMaster{
		kClient:           kClient,
		networkClient:     networkClient,
		kubeInformers:     kubeInformers,
		networkInformers:  networkInformers,
		hostSubnetNodeIPs: map[ktypes.UID]string{},
	}

	var err error
	var clusterNetworkEntries []networkapi.ClusterNetworkEntry
	for _, entry := range networkConfig.ClusterNetworks {
		clusterNetworkEntries = append(clusterNetworkEntries, networkapi.ClusterNetworkEntry{CIDR: entry.CIDR, HostSubnetLength: entry.HostSubnetLength})
	}
	master.networkInfo, err = common.ParseNetworkInfo(clusterNetworkEntries, networkConfig.ServiceNetworkCIDR)
	if err != nil {
		return err
	}
	if len(clusterNetworkEntries) == 0 {
		panic("No ClusterNetworks set in networkConfig; should have been defaulted in if not configured")
	}

	var parsedClusterNetworkEntries []networkapi.ClusterNetworkEntry
	for _, entry := range master.networkInfo.ClusterNetworks {
		parsedClusterNetworkEntries = append(parsedClusterNetworkEntries, networkapi.ClusterNetworkEntry{CIDR: entry.ClusterCIDR.String(), HostSubnetLength: entry.HostSubnetLength})
	}

	configCN := &networkapi.ClusterNetwork{
		TypeMeta:   metav1.TypeMeta{Kind: "ClusterNetwork"},
		ObjectMeta: metav1.ObjectMeta{Name: networkapi.ClusterNetworkDefault},

		ClusterNetworks: parsedClusterNetworkEntries,
		ServiceNetwork:  master.networkInfo.ServiceNetwork.String(),
		PluginName:      networkConfig.NetworkPluginName,

		// Need to set these for backward compat
		Network:          parsedClusterNetworkEntries[0].CIDR,
		HostSubnetLength: parsedClusterNetworkEntries[0].HostSubnetLength,
	}
	osapivalidation.SetDefaultClusterNetwork(*configCN)

	// try this for a while before just dying
	var getError error
	err = wait.PollImmediate(1*time.Second, time.Minute, func() (bool, error) {
		// reset this so that failures come through correctly.
		getError = nil
		existingCN, err := master.networkClient.Network().ClusterNetworks().Get(networkapi.ClusterNetworkDefault, metav1.GetOptions{})
		if err != nil {
			if !kapierrors.IsNotFound(err) {
				// the first request can fail on permissions
				getError = err
				return false, nil
			}
			if err = master.checkClusterNetworkAgainstLocalNetworks(); err != nil {
				return false, err
			}

			if _, err = master.networkClient.Network().ClusterNetworks().Create(configCN); err != nil {
				return false, err
			}
			glog.Infof("Created ClusterNetwork %s", common.ClusterNetworkToString(configCN))

			if err = master.checkClusterNetworkAgainstClusterObjects(); err != nil {
				utilruntime.HandleError(fmt.Errorf("Cluster contains objects incompatible with new ClusterNetwork: %v", err))
			}
		} else {
			configChanged, err := clusterNetworkChanged(configCN, existingCN)
			if err != nil {
				return false, err
			}
			if configChanged {
				configCN.TypeMeta = existingCN.TypeMeta
				configCN.ObjectMeta = existingCN.ObjectMeta
				if err = master.checkClusterNetworkAgainstClusterObjects(); err != nil {
					utilruntime.HandleError(fmt.Errorf("Attempting to modify cluster to exclude existing objects: %v", err))
					return false, err
				}
				if _, err = master.networkClient.Network().ClusterNetworks().Update(configCN); err != nil {
					return false, err
				}
				glog.Infof("Updated ClusterNetwork %s", common.ClusterNetworkToString(configCN))
			} else {
				glog.V(5).Infof("No change to ClusterNetwork %s", common.ClusterNetworkToString(configCN))
			}
		}

		return true, nil
	})
	if err != nil {
		if getError != nil {
			return getError
		}
		return err
	}

	if err = master.SubnetStartMaster(master.networkInfo.ClusterNetworks); err != nil {
		return err
	}

	switch networkConfig.NetworkPluginName {
	case network.MultiTenantPluginName:
		master.vnids = newMasterVNIDMap(true)
		if err = master.VnidStartMaster(); err != nil {
			return err
		}
	case network.NetworkPolicyPluginName:
		master.vnids = newMasterVNIDMap(false)
		if err = master.VnidStartMaster(); err != nil {
			return err
		}
	}

	return nil
}

func (master *OsdnMaster) checkClusterNetworkAgainstLocalNetworks() error {
	hostIPNets, _, err := netutils.GetHostIPNetworks([]string{tun0})
	if err != nil {
		return err
	}
	return master.networkInfo.CheckHostNetworks(hostIPNets)
}

func (master *OsdnMaster) checkClusterNetworkAgainstClusterObjects() error {
	var subnets []networkapi.HostSubnet
	var pods []kapi.Pod
	var services []kapi.Service
	if subnetList, err := master.networkClient.Network().HostSubnets().List(metav1.ListOptions{}); err == nil {
		subnets = subnetList.Items
	}
	if podList, err := master.kClient.Core().Pods(metav1.NamespaceAll).List(metav1.ListOptions{}); err == nil {
		pods = podList.Items
	}
	if serviceList, err := master.kClient.Core().Services(metav1.NamespaceAll).List(metav1.ListOptions{}); err == nil {
		services = serviceList.Items
	}

	return master.networkInfo.CheckClusterObjects(subnets, pods, services)
}

func clusterNetworkChanged(obj *networkapi.ClusterNetwork, old *networkapi.ClusterNetwork) (bool, error) {

	if old.ServiceNetwork != obj.ServiceNetwork {
		return true, fmt.Errorf("cannot change the serviceNetworkCIDR of an already-deployed cluster")
	} else if old.PluginName != obj.PluginName {
		return true, nil
	} else if len(old.ClusterNetworks) != len(obj.ClusterNetworks) {
		return true, nil
	} else {
		changed := false
		for _, oldCIDR := range old.ClusterNetworks {
			found := false
			for _, newCIDR := range obj.ClusterNetworks {
				if newCIDR.CIDR == oldCIDR.CIDR && newCIDR.HostSubnetLength == oldCIDR.HostSubnetLength {
					found = true
					break
				}
			}
			if !found {
				changed = true
				break
			}
		}
		return changed, nil

	}
}
