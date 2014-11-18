package kubernetes

import (
	"fmt"
	"net"
	"time"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/controller"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/master"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/service"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/tools"
	kubeutil "github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/GoogleCloudPlatform/kubernetes/plugin/pkg/scheduler"
	"github.com/GoogleCloudPlatform/kubernetes/plugin/pkg/scheduler/factory"
	"github.com/golang/glog"

	"github.com/openshift/origin/pkg/cmd/util"
)

const (
	KubeAPIPrefixV1Beta1 = "/api/v1beta1"
	KubeAPIPrefixV1Beta2 = "/api/v1beta2"
)

// MasterConfig defines the required values to start a Kubernetes master
type MasterConfig struct {
	NodeHosts []string
	PortalNet *net.IPNet

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
	//podInfoGetter := &kclient.HTTPPodInfoGetter{
	//	Client: http.DefaultClient,
	//	Port:   uint(NodePort),
	//}

	kubeletClient, err := kclient.NewKubeletClient(
		&kclient.KubeletConfig{
			Port: 10250,
		},
	)

	if err != nil {
		glog.Fatalf("Unable to configure Kubelet client: %v", err)
	}

	masterConfig := &master.Config{
		Client:             c.KubeClient,
		EtcdHelper:         c.EtcdHelper,
		HealthCheckMinions: true,
		// TODO: https://github.com/GoogleCloudPlatform/kubernetes/commit/019b7fc74c999c1ae8d54c6687735ad54e9b2b68
		// Minions:            c.NodeHosts,
		// PodInfoGetter: podInfoGetter,
		PortalNet:     c.PortalNet,
		KubeletClient: kubeletClient,
		APIPrefix:     "/api", // TODO check, this should not be needed but makes a "panic: http: invalid pattern"
	}
	m := master.New(masterConfig)

	apiserver.NewAPIGroup(m.API_v1beta1()).InstallREST(mux, KubeAPIPrefixV1Beta1)
	apiserver.NewAPIGroup(m.API_v1beta2()).InstallREST(mux, KubeAPIPrefixV1Beta2)

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

	glog.Infof("Started Kubernetes Replication Manager")
}

// RunScheduler starts the Kubernetes scheduler
func (c *MasterConfig) RunScheduler() {
	configFactory := &factory.ConfigFactory{Client: c.KubeClient}
	config := configFactory.Create()
	s := scheduler.New(config)
	s.Run()
	glog.Infof("Started Kubernetes Scheduler")
}
