// +build linux

package master

import (
	"fmt"
	"net"
	"time"

	log "github.com/golang/glog"

	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ktypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	kapi "k8s.io/kubernetes/pkg/api"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	kinternalinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/internalversion"

	osconfigapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/network"
	networkapi "github.com/openshift/origin/pkg/network/apis/network"
	osapivalidation "github.com/openshift/origin/pkg/network/apis/network/validation"
	"github.com/openshift/origin/pkg/network/common"
	networkclient "github.com/openshift/origin/pkg/network/generated/internalclientset"
	"github.com/openshift/origin/pkg/network/node"
	"github.com/openshift/origin/pkg/util/netutils"
)

type OsdnMaster struct {
	kClient         kclientset.Interface
	networkClient   networkclient.Interface
	networkInfo     *common.NetworkInfo
	subnetAllocator *netutils.SubnetAllocator
	vnids           *masterVNIDMap
	informers       kinternalinformers.SharedInformerFactory

	// Holds Node IP used in creating host subnet for a node
	hostSubnetNodeIPs map[ktypes.UID]string
}

func Start(networkConfig osconfigapi.MasterNetworkConfig, networkClient networkclient.Interface, kClient kclientset.Interface, informers kinternalinformers.SharedInformerFactory) error {
	if !network.IsOpenShiftNetworkPlugin(networkConfig.NetworkPluginName) {
		return nil
	}

	log.Infof("Initializing SDN master of type %q", networkConfig.NetworkPluginName)

	master := &OsdnMaster{
		kClient:           kClient,
		networkClient:     networkClient,
		informers:         informers,
		hostSubnetNodeIPs: map[ktypes.UID]string{},
	}

	var err error
	master.networkInfo, err = common.ParseNetworkInfo(networkConfig.ClusterNetworkCIDR, networkConfig.ServiceNetworkCIDR)
	if err != nil {
		return err
	}

	configCN := &networkapi.ClusterNetwork{
		TypeMeta:   metav1.TypeMeta{Kind: "ClusterNetwork"},
		ObjectMeta: metav1.ObjectMeta{Name: networkapi.ClusterNetworkDefault},

		Network:          networkConfig.ClusterNetworkCIDR,
		HostSubnetLength: networkConfig.HostSubnetLength,
		ServiceNetwork:   networkConfig.ServiceNetworkCIDR,
		PluginName:       networkConfig.NetworkPluginName,
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
			log.Infof("Created ClusterNetwork %s", common.ClusterNetworkToString(configCN))

			if err = master.checkClusterNetworkAgainstClusterObjects(); err != nil {
				log.Errorf("WARNING: cluster contains objects incompatible with new ClusterNetwork: %v", err)
			}
		} else {
			configChanged, err := clusterNetworkChanged(configCN, existingCN)
			if err != nil {
				return false, err
			}
			if configChanged {
				configCN.TypeMeta = existingCN.TypeMeta
				configCN.ObjectMeta = existingCN.ObjectMeta
				if _, err = master.networkClient.Network().ClusterNetworks().Update(configCN); err != nil {
					return false, err
				}
				log.Infof("Updated ClusterNetwork %s", common.ClusterNetworkToString(configCN))
			} else {
				log.V(5).Infof("No change to ClusterNetwork %s", common.ClusterNetworkToString(configCN))
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

	if err = master.SubnetStartMaster(master.networkInfo.ClusterNetwork, networkConfig.HostSubnetLength); err != nil {
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
	hostIPNets, _, err := netutils.GetHostIPNetworks([]string{node.Tun0})
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
	changed := false

	if old.Network != obj.Network {
		changed = true

		_, newNet, err := net.ParseCIDR(obj.Network)
		if err != nil {
			return true, err
		}
		newSize, _ := newNet.Mask.Size()
		oldBase, oldNet, err := net.ParseCIDR(old.Network)
		if err != nil {
			// Shouldn't happen, but if the existing value is invalid, then any change should be an improvement...
		} else {
			oldSize, _ := oldNet.Mask.Size()

			// oldSize and newSize are, eg the "16" in "10.1.0.0/16", so
			// "newSize < oldSize" means the new network is larger
			if !(newSize < oldSize && newNet.Contains(oldBase)) {
				return true, fmt.Errorf("cannot change clusterNetworkCIDR to a value that does not include the existing network.")
			}
		}
	}
	if old.HostSubnetLength != obj.HostSubnetLength {
		return true, fmt.Errorf("cannot change the hostSubnetLength of an already-deployed cluster")
	}
	if old.ServiceNetwork != obj.ServiceNetwork {
		return true, fmt.Errorf("cannot change the serviceNetworkCIDR of an already-deployed cluster")
	}
	if old.PluginName != obj.PluginName {
		changed = true
	}

	return changed, nil
}
