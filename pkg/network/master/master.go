package master

import (
	"fmt"
	"time"

	"github.com/golang/glog"

	kapi "k8s.io/api/core/v1"
	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ktypes "k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	kcoreinformers "k8s.io/client-go/informers/core/v1"
	kclientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	networkapi "github.com/openshift/api/network/v1"
	networkclient "github.com/openshift/client-go/network/clientset/versioned"
	networkinternalinformers "github.com/openshift/client-go/network/informers/externalversions"
	networkinformers "github.com/openshift/client-go/network/informers/externalversions/network/v1"
	osconfigapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	"github.com/openshift/origin/pkg/network"
	"github.com/openshift/origin/pkg/network/common"
	"github.com/openshift/origin/pkg/util/netutils"
)

const (
	tun0 = "tun0"
)

type OsdnMaster struct {
	kClient       kclientset.Interface
	networkClient networkclient.Interface
	networkInfo   *common.NetworkInfo
	vnids         *masterVNIDMap

	nodeInformer         kcoreinformers.NodeInformer
	namespaceInformer    kcoreinformers.NamespaceInformer
	hostSubnetInformer   networkinformers.HostSubnetInformer
	netNamespaceInformer networkinformers.NetNamespaceInformer

	// Used for allocating subnets in order
	subnetAllocatorList []*SubnetAllocator
	// Used for clusterNetwork --> subnetAllocator lookup
	subnetAllocatorMap map[common.ClusterNetwork]*SubnetAllocator

	// Holds Node IP used in creating host subnet for a node
	hostSubnetNodeIPs map[ktypes.UID]string
}

func Start(networkConfig osconfigapi.NetworkControllerConfig, networkClient networkclient.Interface,
	kClient kclientset.Interface, kubeInformers informers.SharedInformerFactory,
	networkInformers networkinternalinformers.SharedInformerFactory) error {
	glog.Infof("Initializing SDN master of type %q", networkConfig.NetworkPluginName)

	master := &OsdnMaster{
		kClient:       kClient,
		networkClient: networkClient,

		nodeInformer:         kubeInformers.Core().V1().Nodes(),
		namespaceInformer:    kubeInformers.Core().V1().Namespaces(),
		hostSubnetInformer:   networkInformers.Network().V1().HostSubnets(),
		netNamespaceInformer: networkInformers.Network().V1().NetNamespaces(),

		subnetAllocatorMap: map[common.ClusterNetwork]*SubnetAllocator{},
		hostSubnetNodeIPs:  map[ktypes.UID]string{},
	}

	var err error
	var clusterNetworkEntries []networkapi.ClusterNetworkEntry
	for _, entry := range networkConfig.ClusterNetworks {
		clusterNetworkEntries = append(clusterNetworkEntries, networkapi.ClusterNetworkEntry{CIDR: entry.CIDR, HostSubnetLength: entry.HostSubnetLength})
	}
	master.networkInfo, err = common.ParseNetworkInfo(clusterNetworkEntries, networkConfig.ServiceNetworkCIDR, &networkConfig.VXLANPort)
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
		VXLANPort:       &networkConfig.VXLANPort,

		// Need to set these for backward compat
		Network:          parsedClusterNetworkEntries[0].CIDR,
		HostSubnetLength: parsedClusterNetworkEntries[0].HostSubnetLength,
	}

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

	// FIXME: this is required to register informers for the types we care about to ensure the informers are started.
	// FIXME: restructure this controller to add event handlers in Start() before returning, instead of inside startSubSystems.
	master.nodeInformer.Informer().GetController()
	master.namespaceInformer.Informer().GetController()
	master.hostSubnetInformer.Informer().GetController()
	master.netNamespaceInformer.Informer().GetController()

	go master.startSubSystems(networkConfig.NetworkPluginName)

	return nil
}

func (master *OsdnMaster) startSubSystems(pluginName string) {
	// Wait for informer sync
	if !cache.WaitForCacheSync(wait.NeverStop,
		master.nodeInformer.Informer().GetController().HasSynced,
		master.namespaceInformer.Informer().GetController().HasSynced,
		master.hostSubnetInformer.Informer().GetController().HasSynced,
		master.netNamespaceInformer.Informer().GetController().HasSynced) {
		glog.Fatalf("failed to sync SDN master informers")
	}

	if err := master.startSubnetMaster(); err != nil {
		glog.Fatalf("failed to start subnet master: %v", err)
	}

	switch pluginName {
	case network.MultiTenantPluginName:
		master.vnids = newMasterVNIDMap(true)
	case network.NetworkPolicyPluginName:
		master.vnids = newMasterVNIDMap(false)
	}
	if master.vnids != nil {
		if err := master.startVNIDMaster(); err != nil {
			glog.Fatalf("failed to start VNID master: %v", err)
		}
	}

	eim := newEgressIPManager()
	eim.Start(master.networkClient, master.hostSubnetInformer, master.netNamespaceInformer)
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
