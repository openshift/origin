package flatsdn

import (
	"github.com/golang/glog"

	"github.com/openshift/openshift-sdn/plugins/osdn"
	"github.com/openshift/openshift-sdn/plugins/osdn/api"
	oskserver "github.com/openshift/origin/pkg/cmd/server/kubernetes"

	knetwork "k8s.io/kubernetes/pkg/kubelet/network"
	kubeletTypes "k8s.io/kubernetes/pkg/kubelet/types"
	utilexec "k8s.io/kubernetes/pkg/util/exec"
)

type flatsdnPlugin struct {
	osdn.OvsController
}

func NetworkPluginName() string {
	return "redhat/openshift-ovs-subnet"
}

func CreatePlugin(registry *osdn.Registry, hostname string, selfIP string, ready chan struct{}) (api.OsdnPlugin, oskserver.FilteringEndpointsConfigHandler, error) {
	fsp := &flatsdnPlugin{}

	err := fsp.BaseInit(registry, NewFlowController(), fsp, hostname, selfIP, ready)
	if err != nil {
		return nil, nil, err
	}

	return fsp, nil, err
}

func (plugin *flatsdnPlugin) PluginStartMaster(clusterNetworkCIDR string, clusterBitsPerSubnet uint, serviceNetworkCIDR string) error {
	if err := plugin.SubnetStartMaster(clusterNetworkCIDR, clusterBitsPerSubnet, serviceNetworkCIDR); err != nil {
		return err
	}

	return nil
}

func (plugin *flatsdnPlugin) PluginStartNode(mtu uint) error {
	if err := plugin.SubnetStartNode(mtu); err != nil {
		return err
	}

	return nil
}

//-----------------------------------------------

const (
	setUpCmd    = "setup"
	tearDownCmd = "teardown"
	statusCmd   = "status"
)

func (plugin *flatsdnPlugin) getExecutable() string {
	return "openshift-ovs-subnet"
}

func (plugin *flatsdnPlugin) Init(host knetwork.Host) error {
	return nil
}

func (plugin *flatsdnPlugin) Name() string {
	return NetworkPluginName()
}

func (plugin *flatsdnPlugin) SetUpPod(namespace string, name string, id kubeletTypes.DockerID) error {
	out, err := utilexec.New().Command(plugin.getExecutable(), setUpCmd, namespace, name, string(id)).CombinedOutput()
	glog.V(5).Infof("SetUpPod 'flatsdn' network plugin output: %s, %v", string(out), err)
	return err
}

func (plugin *flatsdnPlugin) TearDownPod(namespace string, name string, id kubeletTypes.DockerID) error {
	out, err := utilexec.New().Command(plugin.getExecutable(), tearDownCmd, namespace, name, string(id)).CombinedOutput()
	glog.V(5).Infof("TearDownPod 'flatsdn' network plugin output: %s, %v", string(out), err)
	return err
}

func (plugin *flatsdnPlugin) Status(namespace string, name string, id kubeletTypes.DockerID) (*knetwork.PodNetworkStatus, error) {
	return nil, nil
}
