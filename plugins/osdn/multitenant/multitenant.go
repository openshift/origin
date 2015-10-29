package multitenant

import (
	"github.com/golang/glog"

	knetwork "k8s.io/kubernetes/pkg/kubelet/network"

	"github.com/openshift/openshift-sdn/plugins/osdn"
)

func NetworkPluginName() string {
	return "redhat/openshift-ovs-multitenant"
}

func init() {
	osdn.RegisterPlugin(NetworkPluginName(), createPlugin)
}

func createPlugin(registry *osdn.Registry, hostname string, selfIP string, ready chan struct{}) (*osdn.OvsController, error) {
	controller, err := osdn.NewBaseController(registry, NewFlowController(), hostname, selfIP, ready)
	if err != nil {
		return nil, err
	}

	controller.AddStartMasterFunc(osdn.SubnetStartMaster)
	controller.AddStartMasterFunc(osdn.VnidStartMaster)
	controller.AddStartNodeFunc(osdn.SubnetStartNode)
	controller.AddStartNodeFunc(osdn.VnidStartNode)
	return controller, err
}

func Master(registry *osdn.Registry, clusterNetworkCIDR string, clusterBitsPerSubnet uint, serviceNetworkCIDR string) {
	kc, err := osdn.NewController(NetworkPluginName(), registry, "", "", nil)
	if err != nil {
		glog.Fatalf("SDN initialization failed: %v", err)
	}
	err = kc.StartMaster(clusterNetworkCIDR, clusterBitsPerSubnet, serviceNetworkCIDR)
	if err != nil {
		glog.Fatalf("SDN initialization failed: %v", err)
	}
}

func Node(registry *osdn.Registry, hostname string, publicIP string, ready chan struct{}, plugin knetwork.NetworkPlugin, mtu uint) {
	mp, ok := plugin.(*MultitenantPlugin)
	if !ok {
		glog.Fatalf("Failed to type cast provided plugin to a multitenant type plugin")
	}
	kc, err := osdn.NewController(NetworkPluginName(), registry, hostname, publicIP, ready)
	if err != nil {
		glog.Fatalf("SDN initialization failed: %v", err)
	}
	mp.OvsController = kc
	err = kc.StartNode(mtu)
	if err != nil {
		glog.Fatalf("SDN Node failed: %v", err)
	}
}
