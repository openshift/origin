package flatsdn

import (
	"github.com/golang/glog"

	"github.com/openshift/openshift-sdn/plugins/osdn"
)

func NetworkPluginName() string {
	return "redhat/openshift-ovs-subnet"
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
	controller.AddStartNodeFunc(osdn.SubnetStartNode)
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

func Node(registry *osdn.Registry, hostname string, publicIP string, ready chan struct{}, mtu uint) {
	kc, err := osdn.NewController(NetworkPluginName(), registry, hostname, publicIP, ready)
	if err != nil {
		glog.Fatalf("SDN initialization failed: %v", err)
	}
	err = kc.StartNode(mtu)
	if err != nil {
		glog.Fatalf("SDN Node failed: %v", err)
	}
}
