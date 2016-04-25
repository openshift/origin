package ovs

import (
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"

	"github.com/golang/glog"

	"github.com/openshift/openshift-sdn/plugins/osdn"
	"github.com/openshift/openshift-sdn/plugins/osdn/api"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/resource"
	kubeletTypes "k8s.io/kubernetes/pkg/kubelet/container"
	knetwork "k8s.io/kubernetes/pkg/kubelet/network"
	utilsets "k8s.io/kubernetes/pkg/util/sets"
)

type ovsPlugin struct {
	osdn.OsdnController

	multitenant bool
}

func SingleTenantPluginName() string {
	return "redhat/openshift-ovs-subnet"
}

func MultiTenantPluginName() string {
	return "redhat/openshift-ovs-multitenant"
}

func CreatePlugin(registry *osdn.Registry, multitenant bool, hostname string, selfIP string) (api.OsdnPlugin, api.FilteringEndpointsConfigHandler, error) {
	plugin := &ovsPlugin{multitenant: multitenant}

	err := plugin.BaseInit(registry, plugin, multitenant, hostname, selfIP)
	if err != nil {
		return nil, nil, err
	}

	if multitenant {
		return plugin, registry, err
	} else {
		return plugin, nil, err
	}
}

func (plugin *ovsPlugin) PluginStartMaster(clusterNetwork *net.IPNet, hostSubnetLength uint) error {
	if err := plugin.SubnetStartMaster(clusterNetwork, hostSubnetLength); err != nil {
		return err
	}

	if plugin.multitenant {
		if err := plugin.VnidStartMaster(); err != nil {
			return err
		}
	}

	return nil
}

func (plugin *ovsPlugin) PluginStartNode(mtu uint) error {
	networkChanged, err := plugin.SubnetStartNode(mtu)
	if err != nil {
		return err
	}

	if plugin.multitenant {
		if err := plugin.VnidStartNode(); err != nil {
			return err
		}
	}

	if networkChanged {
		pods, err := plugin.GetLocalPods(kapi.NamespaceAll)
		if err != nil {
			return err
		}
		for _, p := range pods {
			containerID := osdn.GetPodContainerID(&p)
			err = plugin.UpdatePod(p.Namespace, p.Name, kubeletTypes.DockerID(containerID))
			if err != nil {
				glog.Warningf("Could not update pod %q (%s): %s", p.Name, containerID, err)
			}
		}
	}

	return nil
}

//-----------------------------------------------

const (
	setUpCmd    = "setup"
	tearDownCmd = "teardown"
	statusCmd   = "status"
	updateCmd   = "update"
)

func (plugin *ovsPlugin) getExecutable() string {
	return "openshift-sdn-ovs"
}

func (plugin *ovsPlugin) Init(host knetwork.Host) error {
	return nil
}

func (plugin *ovsPlugin) Name() string {
	if plugin.multitenant {
		return MultiTenantPluginName()
	} else {
		return SingleTenantPluginName()
	}
}

func (plugin *ovsPlugin) Capabilities() utilsets.Int {
	return utilsets.NewInt(knetwork.NET_PLUGIN_CAPABILITY_SHAPING)
}

func (plugin *ovsPlugin) getVNID(namespace string) (string, error) {
	if plugin.multitenant {
		vnid, found := plugin.VNIDMap[namespace]
		if !found {
			return "", fmt.Errorf("Error fetching VNID for namespace: %s", namespace)
		}
		return strconv.FormatUint(uint64(vnid), 10), nil
	}

	return "0", nil
}

var minRsrc = resource.MustParse("1k")
var maxRsrc = resource.MustParse("1P")

func parseAndValidateBandwidth(value string) (int64, error) {
	rsrc, err := resource.ParseQuantity(value)
	if err != nil {
		return -1, err
	}

	if rsrc.Value() < minRsrc.Value() {
		return -1, fmt.Errorf("resource value %d is unreasonably small (< %d)", rsrc.Value(), minRsrc.Value())
	}
	if rsrc.Value() > maxRsrc.Value() {
		return -1, fmt.Errorf("resource value %d is unreasonably large (> %d)", rsrc.Value(), maxRsrc.Value())
	}
	return rsrc.Value(), nil
}

func extractBandwidthResources(pod *kapi.Pod) (ingress, egress int64, err error) {
	str, found := pod.Annotations["kubernetes.io/ingress-bandwidth"]
	if found {
		ingress, err = parseAndValidateBandwidth(str)
		if err != nil {
			return -1, -1, err
		}
	}
	str, found = pod.Annotations["kubernetes.io/egress-bandwidth"]
	if found {
		egress, err = parseAndValidateBandwidth(str)
		if err != nil {
			return -1, -1, err
		}
	}
	return ingress, egress, nil
}

func wantsMacvlan(pod *kapi.Pod) (bool, error) {
	val, found := pod.Annotations["pod.network.openshift.io/assign-macvlan"]
	if !found || val != "true" {
		return false, nil
	}
	for _, container := range pod.Spec.Containers {
		if container.SecurityContext.Privileged != nil && *container.SecurityContext.Privileged {
			return true, nil
		}
	}
	return false, fmt.Errorf("Pod has 'pod.network.openshift.io/assign-macvlan' annotation but is not privileged")
}

func isScriptError(err error) bool {
	_, ok := err.(*exec.ExitError)
	return ok
}

// Get the last command (which is prefixed with "+" because of "set -x") and its output
func getScriptError(output []byte) string {
	lines := strings.Split(string(output), "\n")
	for n := len(lines) - 1; n >= 0; n-- {
		if strings.HasPrefix(lines[n], "+") {
			return strings.Join(lines[n:], "\n")
		}
	}
	return string(output)
}

func (plugin *ovsPlugin) SetUpPod(namespace string, name string, id kubeletTypes.DockerID) error {
	err := plugin.WaitForPodNetworkReady()
	if err != nil {
		return err
	}

	pod, err := plugin.Registry.GetPod(plugin.HostName, namespace, name)
	if err != nil {
		return err
	}
	if pod == nil {
		return fmt.Errorf("failed to retrieve pod %s/%s", namespace, name)
	}
	ingress, egress, err := extractBandwidthResources(pod)
	if err != nil {
		return fmt.Errorf("failed to parse pod %s/%s ingress/egress quantity: %v", namespace, name, err)
	}
	var ingressStr, egressStr string
	if ingress > 0 {
		ingressStr = fmt.Sprintf("%d", ingress)
	}
	if egress > 0 {
		egressStr = fmt.Sprintf("%d", egress)
	}

	vnidstr, err := plugin.getVNID(namespace)
	if err != nil {
		return err
	}

	macvlan, err := wantsMacvlan(pod)
	if err != nil {
		return err
	}

	out, err := exec.Command(plugin.getExecutable(), setUpCmd, string(id), vnidstr, ingressStr, egressStr, fmt.Sprintf("%t", macvlan)).CombinedOutput()
	glog.V(5).Infof("SetUpPod network plugin output: %s, %v", string(out), err)

	if isScriptError(err) {
		return fmt.Errorf("Error running network setup script: %s", getScriptError(out))
	} else {
		return err
	}
}

func (plugin *ovsPlugin) TearDownPod(namespace string, name string, id kubeletTypes.DockerID) error {
	// The script's teardown functionality doesn't need the VNID
	out, err := exec.Command(plugin.getExecutable(), tearDownCmd, string(id), "-1", "-1", "-1").CombinedOutput()
	glog.V(5).Infof("TearDownPod network plugin output: %s, %v", string(out), err)

	if isScriptError(err) {
		return fmt.Errorf("Error running network teardown script: %s", getScriptError(out))
	} else {
		return err
	}
}

func (plugin *ovsPlugin) Status(namespace string, name string, id kubeletTypes.DockerID) (*knetwork.PodNetworkStatus, error) {
	return nil, nil
}

func (plugin *ovsPlugin) UpdatePod(namespace string, name string, id kubeletTypes.DockerID) error {
	vnidstr, err := plugin.getVNID(namespace)
	if err != nil {
		return err
	}

	out, err := exec.Command(plugin.getExecutable(), updateCmd, string(id), vnidstr).CombinedOutput()
	glog.V(5).Infof("UpdatePod network plugin output: %s, %v", string(out), err)

	if isScriptError(err) {
		return fmt.Errorf("Error running network update script: %s", getScriptError(out))
	} else {
		return err
	}
}

func (plugin *ovsPlugin) Event(name string, details map[string]interface{}) {
}
