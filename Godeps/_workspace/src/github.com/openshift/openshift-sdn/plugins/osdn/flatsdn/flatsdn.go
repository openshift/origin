package flatsdn

import (
	"github.com/golang/glog"
	"strings"

	"k8s.io/kubernetes/pkg/util/exec"

	"github.com/openshift/openshift-sdn/pkg/ovssubnet"
	"github.com/openshift/openshift-sdn/plugins/osdn"
)

func NetworkPluginName() string {
	return "redhat/openshift-ovs-subnet"
}

func Master(registry *osdn.OsdnRegistryInterface, clusterNetworkCIDR string, clusterBitsPerSubnet uint, serviceNetworkCIDR string) {
	// get hostname from the gateway
	output, err := exec.New().Command("uname", "-n").CombinedOutput()
	if err != nil {
		glog.Fatalf("SDN initialization failed: %v", err)
	}
	host := strings.TrimSpace(string(output))

	kc, err := ovssubnet.NewKubeController(registry, host, "", nil)
	if err != nil {
		glog.Fatalf("SDN initialization failed: %v", err)
	}
	err = kc.StartMaster(clusterNetworkCIDR, clusterBitsPerSubnet, serviceNetworkCIDR)
	if err != nil {
		glog.Fatalf("SDN initialization failed: %v", err)
	}
}

func Node(registry *osdn.OsdnRegistryInterface, hostname string, publicIP string, ready chan struct{}, mtu uint) {
	kc, err := ovssubnet.NewKubeController(registry, hostname, publicIP, ready)
	if err != nil {
		glog.Fatalf("SDN initialization failed: %v", err)
	}
	err = kc.StartNode(mtu)
	if err != nil {
		glog.Fatalf("SDN Node failed: %v", err)
	}
}
