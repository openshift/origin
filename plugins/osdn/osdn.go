package osdn

import (
	"github.com/golang/glog"
	"strings"

	"github.com/openshift/openshift-sdn/ovssubnet"
	osclient "github.com/openshift/origin/pkg/client"

	kclient "k8s.io/kubernetes/pkg/client"
	knetwork "k8s.io/kubernetes/pkg/kubelet/network"
	"k8s.io/kubernetes/pkg/util/exec"
)

const (
	flatSdnNetworkPluginName     = "redhat/openshift-ovs-subnet"
	multitenantNetworkPluginName = "redhat/openshift-ovs-multitenant"
)

func IsOsdnNetworkPlugin(pluginName string) bool {
	return pluginName == flatSdnNetworkPluginName || pluginName == multitenantNetworkPluginName
}

func getController(pluginName string, osClient *osclient.Client, kClient *kclient.Client, hostname string, selfIP string, ready chan struct{}) *ovssubnet.OvsController {
	osdnInterface := NewOsdnRegistry(osClient, kClient)

	var controller *ovssubnet.OvsController
	var err error
	switch pluginName {
	case flatSdnNetworkPluginName:
		controller, err = ovssubnet.NewKubeController(&osdnInterface, hostname, selfIP, ready)
	case multitenantNetworkPluginName:
		controller, err = ovssubnet.NewMultitenantController(&osdnInterface, hostname, selfIP, ready)
	default:
		glog.Fatalf("Not an OSDN plugin '%s'", pluginName)
	}
	if err != nil {
		glog.Fatalf("SDN initialization failed: %v", err)
	}
	return controller
}

func Master(pluginName string, osClient *osclient.Client, kClient *kclient.Client, clusterNetwork string, clusterNetworkLength uint, serviceNetwork string) {
	// get hostname from the gateway
	output, err := exec.New().Command("hostname", "-f").CombinedOutput()
	if err != nil {
		glog.Fatalf("SDN initialization failed: %v", err)
	}
	host := strings.TrimSpace(string(output))

	controller := getController(pluginName, osClient, kClient, host, "", nil)

	controller.AdminNamespaces = append(controller.AdminNamespaces, "default")
	err = controller.StartMaster(false, clusterNetwork, clusterNetworkLength, serviceNetwork)
	if err != nil {
		glog.Fatalf("SDN StartMaster failed: %v", err)
	}
}

func Node(pluginName string, plugin knetwork.NetworkPlugin, osClient *osclient.Client, kClient *kclient.Client, hostname string, selfIP string, mtu uint, ready chan struct{}) {
	controller := getController(pluginName, osClient, kClient, hostname, selfIP, ready)

	if plugin != nil {
		osdnPlugin, ok := plugin.(*NetworkPlugin)
		if !ok {
			glog.Fatalf("Not an OSDN plugin: %v", plugin)
		}
		osdnPlugin.ovsController = controller
	}

	err := controller.StartNode(false, false, mtu)
	if err != nil {
		glog.Fatalf("SDN StartNode failed: %v", err)
	}
}
