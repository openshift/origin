package kubernetes

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"

	"github.com/emicklei/go-restful"
	"github.com/golang/glog"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client"
	"k8s.io/kubernetes/pkg/client/record"
	endpointcontroller "k8s.io/kubernetes/pkg/controller/endpoint"
	namespacecontroller "k8s.io/kubernetes/pkg/controller/namespace"
	nodecontroller "k8s.io/kubernetes/pkg/controller/node"
	volumeclaimbinder "k8s.io/kubernetes/pkg/controller/persistentvolume"
	replicationcontroller "k8s.io/kubernetes/pkg/controller/replication"
	resourcequotacontroller "k8s.io/kubernetes/pkg/controller/resourcequota"
	"k8s.io/kubernetes/pkg/master"
	"k8s.io/kubernetes/pkg/util"
	"k8s.io/kubernetes/pkg/volume"
	"k8s.io/kubernetes/pkg/volume/host_path"
	"k8s.io/kubernetes/pkg/volume/nfs"
	"k8s.io/kubernetes/plugin/pkg/scheduler"
	_ "k8s.io/kubernetes/plugin/pkg/scheduler/algorithmprovider"
	schedulerapi "k8s.io/kubernetes/plugin/pkg/scheduler/api"
	latestschedulerapi "k8s.io/kubernetes/plugin/pkg/scheduler/api/latest"
	"k8s.io/kubernetes/plugin/pkg/scheduler/factory"
)

const (
	KubeAPIPrefix        = "/api"
	KubeAPIPrefixV1Beta3 = "/api/v1beta3"
	KubeAPIPrefixV1      = "/api/v1"
)

// InstallAPI starts a Kubernetes master and registers the supported REST APIs
// into the provided mux, then returns an array of strings indicating what
// endpoints were started (these are format strings that will expect to be sent
// a single string value).
func (c *MasterConfig) InstallAPI(container *restful.Container) []string {
	c.Master.RestfulContainer = container
	_ = master.New(c.Master)

	messages := []string{}
	if c.Master.EnableV1Beta3 {
		messages = append(messages, fmt.Sprintf("Started Kubernetes API at %%s%s (deprecated)", KubeAPIPrefixV1Beta3))
	}
	if !c.Master.DisableV1 {
		messages = append(messages, fmt.Sprintf("Started Kubernetes API at %%s%s", KubeAPIPrefixV1))
	}

	return messages
}

// RunNamespaceController starts the Kubernetes Namespace Manager
func (c *MasterConfig) RunNamespaceController() {
	namespaceController := namespacecontroller.NewNamespaceController(c.KubeClient, c.ControllerManager.NamespaceSyncPeriod)
	namespaceController.Run()
	glog.Infof("Started Kubernetes Namespace Manager")
}

// RunPersistentVolumeClaimBinder starts the Kubernetes Persistent Volume Claim Binder
func (c *MasterConfig) RunPersistentVolumeClaimBinder() {
	binder := volumeclaimbinder.NewPersistentVolumeClaimBinder(c.KubeClient, c.ControllerManager.PVClaimBinderSyncPeriod)
	binder.Run()
	glog.Infof("Started Kubernetes Persistent Volume Claim Binder")
}

func (c *MasterConfig) RunPersistentVolumeClaimRecycler(recyclerImageName string) {
	// VolumeConfig contains defaults that can be overridden
	volumeConfig := volume.NewVolumeConfig()

	defaultScrubPod := volumeConfig.PersistentVolumeRecyclerDefaultScrubPod
	defaultScrubPod.Spec.Containers[0].Image = recyclerImageName
	defaultScrubPod.Spec.Containers[0].Command = []string{"/usr/share/openshift/scripts/volumes/recycler.sh"}
	defaultScrubPod.Spec.Containers[0].Args = []string{"/scrub"}

	volumeConfig.PersistentVolumeRecyclerDefaultScrubPod = defaultScrubPod
	volumeConfig.PersistentVolumeRecyclerMinTimeoutNfs = 300
	volumeConfig.PersistentVolumeRecyclerTimeoutIncrementNfs = 30
	volumeConfig.PersistentVolumeRecyclerMinTimeoutHostPath = 120
	volumeConfig.PersistentVolumeRecyclerTimeoutIncrementHostPath = 30

	allPlugins := []volume.VolumePlugin{}
	allPlugins = append(allPlugins, host_path.ProbeVolumePlugins(volumeConfig)...)
	allPlugins = append(allPlugins, nfs.ProbeVolumePlugins(volumeConfig)...)

	recycler, err := volumeclaimbinder.NewPersistentVolumeRecycler(c.KubeClient, c.ControllerManager.PVClaimBinderSyncPeriod, allPlugins)
	if err != nil {
		glog.Fatalf("Could not start PersistentVolumeRecycler: %+v", err)
	}
	recycler.Run()
	glog.Infof("Started Kubernetes PersistentVolumeRecycler")
}

// RunReplicationController starts the Kubernetes replication controller sync loop
func (c *MasterConfig) RunReplicationController(client *client.Client) {
	controllerManager := replicationcontroller.NewReplicationManager(client, replicationcontroller.BurstReplicas)
	go controllerManager.Run(c.ControllerManager.ConcurrentRCSyncs, util.NeverStop)
	glog.Infof("Started Kubernetes Replication Manager")
}

// RunEndpointController starts the Kubernetes replication controller sync loop
func (c *MasterConfig) RunEndpointController() {
	endpoints := endpointcontroller.NewEndpointController(c.KubeClient)
	go endpoints.Run(c.ControllerManager.ConcurrentEndpointSyncs, util.NeverStop)

	glog.Infof("Started Kubernetes Endpoint Controller")
}

// RunScheduler starts the Kubernetes scheduler
func (c *MasterConfig) RunScheduler() {
	config, err := c.createSchedulerConfig()
	if err != nil {
		glog.Fatalf("Unable to start scheduler: %v", err)
	}
	eventcast := record.NewBroadcaster()
	config.Recorder = eventcast.NewRecorder(kapi.EventSource{Component: "scheduler"})
	eventcast.StartRecordingToSink(c.KubeClient.Events(""))

	s := scheduler.New(config)
	s.Run()
	glog.Infof("Started Kubernetes Scheduler")
}

// RunResourceQuotaManager starts the resource quota manager
func (c *MasterConfig) RunResourceQuotaManager() {
	resourceQuotaManager := resourcequotacontroller.NewResourceQuotaController(c.KubeClient)
	resourceQuotaManager.Run(c.ControllerManager.ResourceQuotaSyncPeriod)
}

// RunNodeController starts the node controller
func (c *MasterConfig) RunNodeController() {
	s := c.ControllerManager
	controller := nodecontroller.NewNodeController(
		c.CloudProvider,
		c.KubeClient,
		s.PodEvictionTimeout,

		nodecontroller.NewPodEvictor(util.NewTokenBucketRateLimiter(s.DeletingPodsQps, s.DeletingPodsBurst)),

		s.NodeMonitorGracePeriod,
		s.NodeStartupGracePeriod,
		s.NodeMonitorPeriod,

		(*net.IPNet)(&s.ClusterCIDR),
		s.AllocateNodeCIDRs,
	)

	controller.Run(s.NodeSyncPeriod)

	glog.Infof("Started Kubernetes Node Controller")
}

func (c *MasterConfig) createSchedulerConfig() (*scheduler.Config, error) {
	var policy schedulerapi.Policy
	var configData []byte

	// TODO make the rate limiter configurable
	configFactory := factory.NewConfigFactory(c.KubeClient, util.NewTokenBucketRateLimiter(15.0, 20))
	if _, err := os.Stat(c.Options.SchedulerConfigFile); err == nil {
		configData, err = ioutil.ReadFile(c.Options.SchedulerConfigFile)
		if err != nil {
			return nil, fmt.Errorf("unable to read scheduler config: %v", err)
		}
		err = latestschedulerapi.Codec.DecodeInto(configData, &policy)
		if err != nil {
			return nil, fmt.Errorf("invalid scheduler configuration: %v", err)
		}

		return configFactory.CreateFromConfig(policy)
	}

	// if the config file isn't provided, use the default provider
	return configFactory.CreateFromProvider(factory.DefaultProvider)
}
