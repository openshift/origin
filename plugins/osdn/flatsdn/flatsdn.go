package flatsdn

import (
	"github.com/golang/glog"
	"strings"

	kclient "k8s.io/kubernetes/pkg/client"
	"k8s.io/kubernetes/pkg/util/exec"

	"github.com/openshift/openshift-sdn/ovssubnet"
	osclient "github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/plugins/osdn"
)

func NetworkPluginName() string {
	return "redhat/openshift-ovs-subnet"
}

func Master(osClient *osclient.Client, kClient *kclient.Client, clusterNetwork string, clusterNetworkLength uint) {
	osdnInterface := osdn.NewOsdnRegistryInterface(osClient, kClient)

	// get hostname from the gateway
	output, err := exec.New().Command("hostname", "-f").CombinedOutput()
	if err != nil {
		glog.Fatalf("SDN initialization failed: %v", err)
	}
	host := strings.TrimSpace(string(output))

	kc, err := ovssubnet.NewKubeController(&osdnInterface, host, "", nil)
	if err != nil {
		glog.Fatalf("SDN initialization failed: %v", err)
	}
	err = kc.StartMaster(false, clusterNetwork, clusterNetworkLength)
	if err != nil {
		glog.Fatalf("SDN initialization failed: %v", err)
	}
}

func Node(osClient *osclient.Client, kClient *kclient.Client, hostname string, publicIP string, ready chan struct{}) {
	osdnInterface := osdn.NewOsdnRegistryInterface(osClient, kClient)
	kc, err := ovssubnet.NewKubeController(&osdnInterface, hostname, publicIP, ready)
	if err != nil {
		glog.Fatalf("SDN initialization failed: %v", err)
	}
	err = kc.StartNode(false, false)
	if err != nil {
		glog.Fatalf("SDN Node failed: %v", err)
	}
}
