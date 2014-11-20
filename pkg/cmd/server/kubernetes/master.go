package kubernetes

import (
	"fmt"
	"net"
	"time"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	minionControllerPkg "github.com/GoogleCloudPlatform/kubernetes/pkg/cloudprovider/controller"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/controller"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/master"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/resources"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/service"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/tools"
	kubeutil "github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/GoogleCloudPlatform/kubernetes/plugin/pkg/scheduler"
	"github.com/GoogleCloudPlatform/kubernetes/plugin/pkg/scheduler/factory"
	"github.com/golang/glog"

	"github.com/openshift/origin/pkg/cmd/util"
)

const (
	KubeAPIPrefix        = "/api"
	KubeAPIPrefixV1Beta1 = "/api/v1beta1"
	KubeAPIPrefixV1Beta2 = "/api/v1beta2"
)

// MasterConfig defines the required values to start a Kubernetes master
type MasterConfig struct {
	MasterHost string
	MasterPort int
	NodeHosts  []string
	PortalNet  *net.IPNet

	EtcdHelper tools.EtcdHelper
	KubeClient *kclient.Client
}

// TODO: Longer term we should read this from some config store, rather than a flag.
func (c *MasterConfig) EnsurePortalFlags() {
	if c.PortalNet == nil {
		glog.Fatal("No --portal-net specified")
	}
}

// InstallAPI starts a Kubernetes master and registers the supported REST APIs
// into the provided mux, then returns an array of strings indicating what
// endpoints were started (these are format strings that will expect to be sent
// a single string value).
func (c *MasterConfig) InstallAPI(mux util.Mux) []string {
	kubeletClient, err := kclient.NewKubeletClient(
		&kclient.KubeletConfig{
			Port: 10250,
		},
	)
	if err != nil {
		glog.Fatalf("Unable to configure Kubelet client: %v", err)
	}

	masterConfig := &master.Config{
		PublicAddress: c.MasterHost,
		ReadWritePort: c.MasterPort,
		ReadOnlyPort:  c.MasterPort,

		Authorizer: apiserver.NewAlwaysAllowAuthorizer(),

		Client:             c.KubeClient,
		EtcdHelper:         c.EtcdHelper,
		HealthCheckMinions: true,

		PortalNet: c.PortalNet,

		KubeletClient: kubeletClient,
		APIPrefix:     KubeAPIPrefix,
	}
	m := master.New(masterConfig)

	mux.Handle("/", m.Handler)

	return []string{
		fmt.Sprintf("Started Kubernetes API at %%s%s", KubeAPIPrefixV1Beta1),
		fmt.Sprintf("Started Kubernetes API at %%s%s", KubeAPIPrefixV1Beta2),
	}
}

// RunReplicationController starts the Kubernetes replication controller sync loop
func (c *MasterConfig) RunReplicationController() {
	controllerManager := controller.NewReplicationManager(c.KubeClient)
	controllerManager.Run(10 * time.Second)
	glog.Infof("Started Kubernetes Replication Manager")
}

// RunEndpointController starts the Kubernetes replication controller sync loop
func (c *MasterConfig) RunEndpointController() {
	endpoints := service.NewEndpointController(c.KubeClient)
	go kubeutil.Forever(func() { endpoints.SyncServiceEndpoints() }, time.Second*10)

	glog.Infof("Started Kubernetes Endpoint Controller")
}

// RunScheduler starts the Kubernetes scheduler
func (c *MasterConfig) RunScheduler() {
	configFactory := &factory.ConfigFactory{Client: c.KubeClient}
	config := configFactory.Create()
	s := scheduler.New(config)
	s.Run()
	glog.Infof("Started Kubernetes Scheduler")
}

func (c *MasterConfig) RunMinionController() {
	nodeResources := &kapi.NodeResources{
		Capacity: kapi.ResourceList{
			resources.CPU:    kubeutil.NewIntOrStringFromInt(int(1000)),
			resources.Memory: kubeutil.NewIntOrStringFromInt(int(3 * 1024 * 1024 * 1024)),
		},
	}

	minionController := minionControllerPkg.NewMinionController(nil, "", c.NodeHosts, nodeResources, c.KubeClient)
	minionController.Run(10 * time.Second)

	glog.Infof("Started Kubernetes Minion Controller")
}
