package flatsdn

import (
	"github.com/golang/glog"

	"github.com/openshift/openshift-sdn/pkg/ovssubnet"
	"github.com/openshift/openshift-sdn/plugins/osdn"
)

func NetworkPluginName() string {
	return "redhat/openshift-ovs-subnet"
}

func Master(registry *osdn.Registry, clusterNetworkCIDR string, clusterBitsPerSubnet uint, serviceNetworkCIDR string) {
	kc, err := ovssubnet.NewKubeController(registry, "", "", nil)
	if err != nil {
		glog.Fatalf("SDN initialization failed: %v", err)
	}
	err = kc.StartMaster(clusterNetworkCIDR, clusterBitsPerSubnet, serviceNetworkCIDR)
	if err != nil {
		glog.Fatalf("SDN initialization failed: %v", err)
	}
}

func Node(registry *osdn.Registry, hostname string, publicIP string, ready chan struct{}, mtu uint) {
	kc, err := ovssubnet.NewKubeController(registry, hostname, publicIP, ready)
	if err != nil {
		glog.Fatalf("SDN initialization failed: %v", err)
	}
	err = kc.StartNode(mtu)
	if err != nil {
		glog.Fatalf("SDN Node failed: %v", err)
	}
}
