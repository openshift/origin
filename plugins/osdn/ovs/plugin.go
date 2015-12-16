package ovs

import (
	"fmt"
	"strconv"

	"github.com/golang/glog"

	"github.com/openshift/openshift-sdn/plugins/osdn"
	"github.com/openshift/openshift-sdn/plugins/osdn/api"

	knetwork "k8s.io/kubernetes/pkg/kubelet/network"
	kubeletTypes "k8s.io/kubernetes/pkg/kubelet/types"
	utilexec "k8s.io/kubernetes/pkg/util/exec"
)

type ovsPlugin struct {
	osdn.OvsController

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

	err := plugin.BaseInit(registry, NewFlowController(multitenant), plugin, hostname, selfIP)
	if err != nil {
		return nil, nil, err
	}

	if multitenant {
		return plugin, registry, err
	} else {
		return plugin, nil, err
	}
}

func (plugin *ovsPlugin) PluginStartMaster(clusterNetworkCIDR string, clusterBitsPerSubnet uint, serviceNetworkCIDR string) error {
	if err := plugin.SubnetStartMaster(clusterNetworkCIDR, clusterBitsPerSubnet, serviceNetworkCIDR); err != nil {
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
	if err := plugin.SubnetStartNode(mtu); err != nil {
		return err
	}

	if plugin.multitenant {
		if err := plugin.VnidStartNode(); err != nil {
			return err
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

func (plugin *ovsPlugin) getVNID(namespace string) (string, error) {
	if plugin.multitenant {
		vnid, found := plugin.VNIDMap[namespace]
		if !found {
			return "", fmt.Errorf("Error fetching VNID for namespace: %s", namespace)
		}
		return strconv.FormatUint(uint64(vnid), 10), nil
	}

	return "-1", nil
}

func (plugin *ovsPlugin) SetUpPod(namespace string, name string, id kubeletTypes.DockerID) error {
	err := plugin.WaitForPodNetworkReady()
	if err != nil {
		return err
	}

	vnidstr, err := plugin.getVNID(namespace)
	if err != nil {
		return err
	}

	out, err := utilexec.New().Command(plugin.getExecutable(), setUpCmd, string(id), vnidstr).CombinedOutput()
	glog.V(5).Infof("SetUpPod network plugin output: %s, %v", string(out), err)
	return err
}

func (plugin *ovsPlugin) TearDownPod(namespace string, name string, id kubeletTypes.DockerID) error {
	// The script's teardown functionality doesn't need the VNID
	out, err := utilexec.New().Command(plugin.getExecutable(), tearDownCmd, string(id), "-1").CombinedOutput()
	glog.V(5).Infof("TearDownPod network plugin output: %s, %v", string(out), err)
	return err
}

func (plugin *ovsPlugin) Status(namespace string, name string, id kubeletTypes.DockerID) (*knetwork.PodNetworkStatus, error) {
	return nil, nil
}

func (plugin *ovsPlugin) UpdatePod(namespace string, name string, id kubeletTypes.DockerID) error {
	vnidstr, err := plugin.getVNID(namespace)
	if err != nil {
		return err
	}

	out, err := utilexec.New().Command(plugin.getExecutable(), updateCmd, string(id), vnidstr).CombinedOutput()
	glog.V(5).Infof("UpdatePod network plugin output: %s, %v", string(out), err)
	return err
}
