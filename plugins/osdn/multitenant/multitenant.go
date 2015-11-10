package multitenant

import (
	"fmt"
	"strconv"

	"github.com/golang/glog"

	"github.com/openshift/openshift-sdn/plugins/osdn"
	"github.com/openshift/openshift-sdn/plugins/osdn/api"
	"github.com/openshift/origin/pkg/cmd/server/kubernetes"

	knetwork "k8s.io/kubernetes/pkg/kubelet/network"
	kubeletTypes "k8s.io/kubernetes/pkg/kubelet/types"
	utilexec "k8s.io/kubernetes/pkg/util/exec"
)

type multitenantPlugin struct {
	osdn.OvsController
}

func NetworkPluginName() string {
	return "redhat/openshift-ovs-multitenant"
}

func CreatePlugin(registry *osdn.Registry, hostname string, selfIP string, ready chan struct{}) (api.OsdnPlugin, error) {
	mtp := &multitenantPlugin{}

	err := mtp.BaseInit(registry, NewFlowController(), mtp, hostname, selfIP, ready)
	if err != nil {
		return nil, err
	}

	return mtp, err
}

func (plugin *multitenantPlugin) PluginStartMaster(clusterNetworkCIDR string, clusterBitsPerSubnet uint, serviceNetworkCIDR string) error {
	if err := plugin.SubnetStartMaster(clusterNetworkCIDR, clusterBitsPerSubnet, serviceNetworkCIDR); err != nil {
		return err
	}

	if err := plugin.VnidStartMaster(); err != nil {
		return err
	}

	return nil
}

func (plugin *multitenantPlugin) PluginStartNode(mtu uint) (kubernetes.FilteringEndpointsConfigHandler, error) {
	if err := plugin.SubnetStartNode(mtu); err != nil {
		return nil, err
	}

	if err := plugin.VnidStartNode(); err != nil {
		return nil, err
	}

	return plugin.Registry, nil
}

//-----------------------------------------------

const (
	setUpCmd    = "setup"
	tearDownCmd = "teardown"
	statusCmd   = "status"
)

func (plugin *multitenantPlugin) getExecutable() string {
	return "openshift-ovs-multitenant"
}

func (plugin *multitenantPlugin) Init(host knetwork.Host) error {
	return nil
}

func (plugin *multitenantPlugin) Name() string {
	return NetworkPluginName()
}

func (plugin *multitenantPlugin) SetUpPod(namespace string, name string, id kubeletTypes.DockerID) error {
	vnid, found := plugin.VNIDMap[namespace]
	if !found {
		return fmt.Errorf("Error fetching VNID for namespace: %s", namespace)
	}
	out, err := utilexec.New().Command(plugin.getExecutable(), setUpCmd, namespace, name, string(id), strconv.FormatUint(uint64(vnid), 10)).CombinedOutput()
	glog.V(5).Infof("SetUpPod 'multitenant' network plugin output: %s, %v", string(out), err)
	return err
}

func (plugin *multitenantPlugin) TearDownPod(namespace string, name string, id kubeletTypes.DockerID) error {
	// The script's teardown functionality doesn't need the VNID
	out, err := utilexec.New().Command(plugin.getExecutable(), tearDownCmd, namespace, name, string(id), "-1").CombinedOutput()
	glog.V(5).Infof("TearDownPod 'multitenant' network plugin output: %s, %v", string(out), err)
	return err
}

func (plugin *multitenantPlugin) Status(namespace string, name string, id kubeletTypes.DockerID) (*knetwork.PodNetworkStatus, error) {
	return nil, nil
}
