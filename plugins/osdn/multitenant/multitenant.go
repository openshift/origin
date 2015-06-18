package multitenant

import (
	"github.com/golang/glog"
	"strings"

	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	knetwork "github.com/GoogleCloudPlatform/kubernetes/pkg/kubelet/network"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util/exec"

	"github.com/openshift/openshift-sdn/ovssubnet"
	osclient "github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/plugins/osdn"
)

func NetworkPluginName() string {
	return "redhat/openshift-ovs-multitenant"
}

func Master(osClient *osclient.Client, kClient *kclient.Client, clusterNetwork string, clusterNetworkLength uint) {
	osdnInterface := osdn.NewOsdnRegistryInterface(osClient, kClient)

	// get hostname from the gateway
	output, err := exec.New().Command("hostname", "-f").CombinedOutput()
	if err != nil {
		glog.Fatalf("SDN initialization failed: %v", err)
	}
	host := strings.TrimSpace(string(output))

	kc, err := ovssubnet.NewMultitenantController(&osdnInterface, host, "", nil)
	if err != nil {
		glog.Fatalf("SDN initialization failed: %v", err)
	}
	err = kc.StartMaster(false, clusterNetwork, clusterNetworkLength)
	if err != nil {
		glog.Fatalf("SDN initialization failed: %v", err)
	}
}

func Node(osClient *osclient.Client, kClient *kclient.Client, hostname string, publicIP string, ready chan struct{}, plugin knetwork.NetworkPlugin) {
	mp, ok := plugin.(*MultitenantPlugin)
	if !ok {
		glog.Fatalf("Failed to type cast provided plugin to a multitenant type plugin")
	}
	osdnInterface := osdn.NewOsdnRegistryInterface(osClient, kClient)
	kc, err := ovssubnet.NewMultitenantController(&osdnInterface, hostname, publicIP, ready)
	if err != nil {
		glog.Fatalf("SDN initialization failed: %v", err)
	}
	mp.OvsController = kc
	err = kc.StartNode(false, false)
	if err != nil {
		glog.Fatalf("SDN Node failed: %v", err)
	}
}
