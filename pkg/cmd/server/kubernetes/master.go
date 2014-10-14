package kubernetes

import (
	"fmt"
	"net/http"
	"time"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	kubeclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/controller"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/master"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/tools"
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

	EtcdHelper tools.EtcdHelper
	KubeClient *kubeclient.Client
}

// InstallAPI starts a Kubernetes master and registers the supported REST APIs
// into the provided mux, then returns an array of strings indicating what
// endpoints were started (these are format strings that will expect to be sent
// a single string value).
func (c *MasterConfig) InstallAPI(mux util.Mux) []string {
	podInfoGetter := &kubeclient.HTTPPodInfoGetter{
		Client: http.DefaultClient,
		Port:   uint(NodePort),
	}

	masterConfig := &master.Config{
		Client:             c.KubeClient,
		EtcdHelper:         c.EtcdHelper,
		HealthCheckMinions: true,
		Minions:            c.NodeHosts,
		PodInfoGetter:      podInfoGetter,
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

// RunReplicationController starts the Kubernetes scheduler
func (c *MasterConfig) RunScheduler() {
	configFactory := &factory.ConfigFactory{Client: c.KubeClient}
	config := configFactory.Create()
	s := scheduler.New(config)
	s.Run()
	glog.Infof("Started Kubernetes Scheduler")
}
