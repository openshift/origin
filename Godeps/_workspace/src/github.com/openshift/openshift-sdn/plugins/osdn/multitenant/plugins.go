package multitenant

import (
	"fmt"
	"strconv"

	"github.com/golang/glog"

	"github.com/openshift/openshift-sdn/pkg/ovssubnet"
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

type MultitenantPlugin struct {
	host          knetwork.Host
	OvsController *ovssubnet.OvsController
}

func GetKubeNetworkPlugin() knetwork.NetworkPlugin {
	return &MultitenantPlugin{}
}

func (plugin *MultitenantPlugin) getExecutable() string {
	return "openshift-ovs-multitenant"
}

func (plugin *MultitenantPlugin) Init(host knetwork.Host) error {
	plugin.host = host
	return nil
}

func (plugin *MultitenantPlugin) Name() string {
	return NetworkPluginName()
}

func (plugin *MultitenantPlugin) SetUpPod(namespace string, name string, id kubeletTypes.DockerID) error {
	vnid, found := plugin.OvsController.VNIDMap[namespace]
	if !found {
		return fmt.Errorf("Error fetching VNID for namespace: %s", namespace)
	}
	out, err := utilexec.New().Command(plugin.getExecutable(), setUpCmd, namespace, name, string(id), strconv.FormatUint(uint64(vnid), 10)).CombinedOutput()
	glog.V(5).Infof("SetUpPod 'multitenant' network plugin output: %s, %v", string(out), err)
	return err
}

func (plugin *MultitenantPlugin) TearDownPod(namespace string, name string, id kubeletTypes.DockerID) error {
	vnid, found := plugin.OvsController.VNIDMap[namespace]
	if !found {
		return fmt.Errorf("Error fetching VNID for namespace: %s", namespace)
	}
	out, err := utilexec.New().Command(plugin.getExecutable(), tearDownCmd, namespace, name, string(id), strconv.FormatUint(uint64(vnid), 10)).CombinedOutput()
	glog.V(5).Infof("TearDownPod 'multitenant' network plugin output: %s, %v", string(out), err)
	return err
}

func (plugin *MultitenantPlugin) Status(namespace string, name string, id kubeletTypes.DockerID) (*knetwork.PodNetworkStatus, error) {
	return nil, nil
}
