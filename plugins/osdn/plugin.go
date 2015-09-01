package osdn

import (
	"encoding/json"
	"strconv"

	"github.com/golang/glog"

	"github.com/openshift/openshift-sdn/ovssubnet"
	knetwork "k8s.io/kubernetes/pkg/kubelet/network"
	kubeletTypes "k8s.io/kubernetes/pkg/kubelet/types"
	utilexec "k8s.io/kubernetes/pkg/util/exec"
)

const (
	initCmd     = "init"
	setUpCmd    = "setup"
	tearDownCmd = "teardown"
	statusCmd   = "status"
)

type NetworkPlugin struct {
	name          string
	host          knetwork.Host
	ovsController *ovssubnet.OvsController
}

func GetNetworkPlugin(pluginName string) knetwork.NetworkPlugin {
	if pluginName != multitenantNetworkPluginName {
		return nil
	}
	return &NetworkPlugin{name: pluginName}
}

func (plugin *NetworkPlugin) getExecutable() string {
	// Skip "redhat/"
	return plugin.name[7:]
}

func (plugin *NetworkPlugin) getVnid(namespace string) (uint, error) {
	// get vnid for the namespace
	vnid, ok := plugin.ovsController.VNIDMap[namespace]
	if !ok {
		// vnid does not exist for this pod, set it to zero (or error?)
		vnid = 0
	}
	return vnid, nil
}

func (plugin *NetworkPlugin) Init(host knetwork.Host) error {
	plugin.host = host
	return nil
}

func (plugin *NetworkPlugin) Name() string {
	return plugin.name
}

func (plugin *NetworkPlugin) SetUpPod(namespace string, name string, id kubeletTypes.DockerID) error {
	vnid, err := plugin.getVnid(namespace)
	if err != nil {
		return err
	}
	out, err := utilexec.New().Command(plugin.getExecutable(), setUpCmd, namespace, name, string(id), strconv.FormatUint(uint64(vnid), 10)).CombinedOutput()
	glog.V(5).Infof("SetUpPod 'multitenant' network plugin output: %s, %v", string(out), err)
	return err
}

func (plugin *NetworkPlugin) TearDownPod(namespace string, name string, id kubeletTypes.DockerID) error {
	vnid, err := plugin.getVnid(namespace)
	out, err := utilexec.New().Command(plugin.getExecutable(), tearDownCmd, namespace, name, string(id), strconv.FormatUint(uint64(vnid), 10)).CombinedOutput()
	glog.V(5).Infof("TearDownPod 'multitenant' network plugin output: %s, %v", string(out), err)
	return err
}

func (plugin *NetworkPlugin) Status(namespace string, name string, id kubeletTypes.DockerID) (*knetwork.PodNetworkStatus, error) {
	vnid, err := plugin.getVnid(namespace)
	if err != nil {
		return nil, err
	}
	out, err := utilexec.New().Command(plugin.getExecutable(), statusCmd, namespace, name, string(id), strconv.FormatUint(uint64(vnid), 10)).CombinedOutput()
	glog.V(5).Infof("PodNetworkStatus 'multitenant' network plugin output: %s, %v", string(out), err)
	if err != nil {
		return nil, err
	}
	var podNetworkStatus knetwork.PodNetworkStatus
	err = json.Unmarshal([]byte(out), &podNetworkStatus)
	if err != nil {
		return nil, err
	}
	return &podNetworkStatus, nil
}
