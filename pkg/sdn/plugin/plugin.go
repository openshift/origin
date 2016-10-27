package plugin

import (
	"fmt"

	"k8s.io/kubernetes/pkg/apis/componentconfig"
	kubeletTypes "k8s.io/kubernetes/pkg/kubelet/container"
	knetwork "k8s.io/kubernetes/pkg/kubelet/network"
	kcni "k8s.io/kubernetes/pkg/kubelet/network/cni"
	utilsets "k8s.io/kubernetes/pkg/util/sets"

	"github.com/golang/glog"
)

// This kubelet network plugin shim only exists to grab the knetwork.Host
// Everything else is simply proxied directly to the kubenet CNI driver.
func (node *OsdnNode) Init(host knetwork.Host, hairpinMode componentconfig.HairpinMode, nonMasqueradeCIDR string, mtu int) error {
	plugins := kcni.ProbeNetworkPlugins(kcni.DefaultNetDir, kcni.DefaultCNIDir)
	if len(plugins) == 0 {
		return fmt.Errorf("openshift-sdn CNI network plugin config could not be found in %v", kcni.DefaultNetDir)
	} else if len(plugins) > 1 {
		return fmt.Errorf("multiple CNI network plugins found; only one can be used")
	}
	node.host = host
	node.kubeletCniPlugin = plugins[0]

	err := node.kubeletCniPlugin.Init(host, hairpinMode, nonMasqueradeCIDR, mtu)

	// Let initial pod updates happen if they need to
	glog.V(5).Infof("openshift-sdn CNI plugin initialized")
	close(node.kubeletInitReady)

	return err
}

func (node *OsdnNode) Name() string {
	return kcni.CNIPluginName
}

func (node *OsdnNode) Capabilities() utilsets.Int {
	return utilsets.NewInt(knetwork.NET_PLUGIN_CAPABILITY_SHAPING)
}

func (node *OsdnNode) SetUpPod(namespace string, name string, id kubeletTypes.ContainerID) error {
	return node.kubeletCniPlugin.SetUpPod(namespace, name, id)
}

func (node *OsdnNode) TearDownPod(namespace string, name string, id kubeletTypes.ContainerID) error {
	return node.kubeletCniPlugin.TearDownPod(namespace, name, id)
}

func (node *OsdnNode) Status() error {
	if err := node.IsPodNetworkReady(); err != nil {
		return err
	}
	return node.kubeletCniPlugin.Status()
}

func (node *OsdnNode) GetPodNetworkStatus(namespace string, name string, id kubeletTypes.ContainerID) (*knetwork.PodNetworkStatus, error) {
	return node.kubeletCniPlugin.GetPodNetworkStatus(namespace, name, id)
}

func (node *OsdnNode) Event(name string, details map[string]interface{}) {
	node.kubeletCniPlugin.Event(name, details)
}
