package multitenant

import (
	"github.com/golang/glog"

	knetwork "k8s.io/kubernetes/pkg/kubelet/network"

	"github.com/openshift/openshift-sdn/pkg/ovssubnet"
	"github.com/openshift/openshift-sdn/plugins/osdn"
)

func NetworkPluginName() string {
	return "redhat/openshift-ovs-multitenant"
}

func Master(registry *osdn.Registry, clusterNetworkCIDR string, clusterBitsPerSubnet uint, serviceNetworkCIDR string) {
	kc, err := ovssubnet.NewMultitenantController(registry, "", "", nil)
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
	kc, err := ovssubnet.NewMultitenantController(registry, hostname, publicIP, ready)
	if err != nil {
		glog.Fatalf("SDN initialization failed: %v", err)
	}
	mp.OvsController = kc
	err = kc.StartNode(mtu)
	if err != nil {
		glog.Fatalf("SDN Node failed: %v", err)
	}
}
