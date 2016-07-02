package plugin

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/golang/glog"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/resource"
	"k8s.io/kubernetes/pkg/apis/componentconfig"
	kubeletTypes "k8s.io/kubernetes/pkg/kubelet/container"
	knetwork "k8s.io/kubernetes/pkg/kubelet/network"
	utilsets "k8s.io/kubernetes/pkg/util/sets"
)

const (
	SingleTenantPluginName string = "redhat/openshift-ovs-subnet"
	MultiTenantPluginName  string = "redhat/openshift-ovs-multitenant"

	IngressBandwidthAnnotation string = "kubernetes.io/ingress-bandwidth"
	EgressBandwidthAnnotation  string = "kubernetes.io/egress-bandwidth"
	AssignMacVlanAnnotation    string = "pod.network.openshift.io/assign-macvlan"
)

func IsOpenShiftNetworkPlugin(pluginName string) bool {
	switch strings.ToLower(pluginName) {
	case SingleTenantPluginName, MultiTenantPluginName:
		return true
	}
	return false
}

func IsOpenShiftMultitenantNetworkPlugin(pluginName string) bool {
	if strings.ToLower(pluginName) == MultiTenantPluginName {
		return true
	}
	return false
}

//-----------------------------------------------

const (
	setUpCmd    = "setup"
	tearDownCmd = "teardown"
	statusCmd   = "status"
	updateCmd   = "update"
)

func (plugin *OsdnNode) getExecutable() string {
	return "openshift-sdn-ovs"
}

func (plugin *OsdnNode) Init(host knetwork.Host, _ componentconfig.HairpinMode, _ string) error {
	return nil
}

func (plugin *OsdnNode) Name() string {
	if plugin.multitenant {
		return MultiTenantPluginName
	} else {
		return SingleTenantPluginName
	}
}

func (plugin *OsdnNode) Capabilities() utilsets.Int {
	return utilsets.NewInt(knetwork.NET_PLUGIN_CAPABILITY_SHAPING)
}

func (plugin *OsdnNode) getVNID(namespace string) (string, error) {
	if plugin.multitenant {
		vnid, err := plugin.vnids.WaitAndGetVNID(namespace)
		if err != nil {
			return "", err
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
	str, found := pod.Annotations[IngressBandwidthAnnotation]
	if found {
		ingress, err = parseAndValidateBandwidth(str)
		if err != nil {
			return -1, -1, err
		}
	}
	str, found = pod.Annotations[EgressBandwidthAnnotation]
	if found {
		egress, err = parseAndValidateBandwidth(str)
		if err != nil {
			return -1, -1, err
		}
	}
	return ingress, egress, nil
}

func wantsMacvlan(pod *kapi.Pod) (bool, error) {
	val, found := pod.Annotations[AssignMacVlanAnnotation]
	if !found || val != "true" {
		return false, nil
	}
	for _, container := range pod.Spec.Containers {
		if container.SecurityContext.Privileged != nil && *container.SecurityContext.Privileged {
			return true, nil
		}
	}
	return false, fmt.Errorf("Pod has %q annotation but is not privileged", AssignMacVlanAnnotation)
}

func isScriptError(err error) bool {
	_, ok := err.(*exec.ExitError)
	return ok
}

// Get the last command (which is prefixed with "+" because of "set -x") and its output
// (Unless the script ended with "echo ...; exit", in which case we just return the
// echoed text.)
func getScriptError(output []byte) string {
	lines := strings.Split(string(output), "\n")
	last := len(lines)
	for n := last - 1; n >= 0; n-- {
		if strings.HasPrefix(lines[n], "+ exit") {
			last = n
		} else if strings.HasPrefix(lines[n], "+ echo") {
			return strings.Join(lines[n+1:last], "\n")
		} else if strings.HasPrefix(lines[n], "+") {
			return strings.Join(lines[n:], "\n")
		}
	}
	return string(output)
}

func (plugin *OsdnNode) SetUpPod(namespace string, name string, id kubeletTypes.ContainerID) error {
	err := plugin.WaitForPodNetworkReady()
	if err != nil {
		return err
	}

	pod, err := plugin.registry.GetPod(plugin.hostName, namespace, name)
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

	out, err := exec.Command(plugin.getExecutable(), setUpCmd, id.ID, vnidstr, ingressStr, egressStr, fmt.Sprintf("%t", macvlan)).CombinedOutput()
	glog.V(5).Infof("SetUpPod network plugin output: %s, %v", string(out), err)

	if isScriptError(err) {
		return fmt.Errorf("Error running network setup script: %s", getScriptError(out))
	} else {
		return err
	}
}

func (plugin *OsdnNode) TearDownPod(namespace string, name string, id kubeletTypes.ContainerID) error {
	// The script's teardown functionality doesn't need the VNID
	out, err := exec.Command(plugin.getExecutable(), tearDownCmd, id.ID, "-1", "-1", "-1").CombinedOutput()
	glog.V(5).Infof("TearDownPod network plugin output: %s, %v", string(out), err)

	if isScriptError(err) {
		return fmt.Errorf("Error running network teardown script: %s", getScriptError(out))
	} else {
		return err
	}
}

func (plugin *OsdnNode) Status() error {
	return nil
}

func (plugin *OsdnNode) GetPodNetworkStatus(namespace string, name string, podInfraContainerID kubeletTypes.ContainerID) (*knetwork.PodNetworkStatus, error) {
	return nil, nil
}

func (plugin *OsdnNode) UpdatePod(namespace string, name string, id kubeletTypes.DockerID) error {
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

func (plugin *OsdnNode) Event(name string, details map[string]interface{}) {
}
