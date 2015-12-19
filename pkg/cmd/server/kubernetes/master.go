package kubernetes

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"

	"github.com/emicklei/go-restful"
	"github.com/golang/glog"

	osclient "github.com/openshift/origin/pkg/client"
	kctrlmgr "k8s.io/kubernetes/cmd/kube-controller-manager/app"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/record"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	endpointcontroller "k8s.io/kubernetes/pkg/controller/endpoint"
	jobcontroller "k8s.io/kubernetes/pkg/controller/job"
	namespacecontroller "k8s.io/kubernetes/pkg/controller/namespace"
	nodecontroller "k8s.io/kubernetes/pkg/controller/node"
	volumeclaimbinder "k8s.io/kubernetes/pkg/controller/persistentvolume"
	podautoscalercontroller "k8s.io/kubernetes/pkg/controller/podautoscaler"
	"k8s.io/kubernetes/pkg/controller/podautoscaler/metrics"
	replicationcontroller "k8s.io/kubernetes/pkg/controller/replication"
	resourcequotacontroller "k8s.io/kubernetes/pkg/controller/resourcequota"
	"k8s.io/kubernetes/pkg/master"
	"k8s.io/kubernetes/pkg/util"
	"k8s.io/kubernetes/pkg/util/io"
	"k8s.io/kubernetes/pkg/volume"
	"k8s.io/kubernetes/pkg/volume/aws_ebs"
	"k8s.io/kubernetes/pkg/volume/cinder"
	"k8s.io/kubernetes/pkg/volume/gce_pd"
	"k8s.io/kubernetes/pkg/volume/host_path"
	"k8s.io/kubernetes/pkg/volume/nfs"
	"k8s.io/kubernetes/plugin/pkg/scheduler"
	_ "k8s.io/kubernetes/plugin/pkg/scheduler/algorithmprovider"
	schedulerapi "k8s.io/kubernetes/plugin/pkg/scheduler/api"
	latestschedulerapi "k8s.io/kubernetes/plugin/pkg/scheduler/api/latest"
	"k8s.io/kubernetes/plugin/pkg/scheduler/factory"
)

const (
	KubeAPIPrefix                  = "/api"
	KubeAPIPrefixV1                = KubeAPIPrefix + "/v1"
	KubeAPIGroupPrefix             = "/apis"
	KubeAPIExtensionsPrefixV1beta1 = KubeAPIGroupPrefix + "/extensions/v1beta1"
)

// InstallAPI starts a Kubernetes master and registers the supported REST APIs
// into the provided mux, then returns an array of strings indicating what
// endpoints were started (these are format strings that will expect to be sent
// a single string value).
func (c *MasterConfig) InstallAPI(container *restful.Container) []string {
	c.Master.RestfulContainer = container
	_ = master.New(c.Master)

	messages := []string{}
	if !c.Master.DisableV1 {
		messages = append(messages, fmt.Sprintf("Started Kubernetes API at %%s%s", KubeAPIPrefixV1))
	}

	if c.Master.EnableExp {
		messages = append(messages, fmt.Sprintf("Started Kubernetes API Extensions at %%s%s", KubeAPIExtensionsPrefixV1beta1))
	}

	return messages
}

// RunNamespaceController starts the Kubernetes Namespace Manager
func (c *MasterConfig) RunNamespaceController() {
	// we now have several of the kube "experimental" pieces enabled in Origin, so this needs to be
	// enabled whenever we have the "experimental" APIs enabled.
	experimentalMode := c.Master.EnableExp
	namespaceController := namespacecontroller.NewNamespaceController(c.KubeClient, experimentalMode, c.ControllerManager.NamespaceSyncPeriod)
	namespaceController.Run()
}

// RunPersistentVolumeClaimBinder starts the Kubernetes Persistent Volume Claim Binder
func (c *MasterConfig) RunPersistentVolumeClaimBinder(client *client.Client) {
	binder := volumeclaimbinder.NewPersistentVolumeClaimBinder(client, c.ControllerManager.PVClaimBinderSyncPeriod)
	binder.Run()
}

func (c *MasterConfig) RunPersistentVolumeProvisioner(client *client.Client) {
	provisioner, err := kctrlmgr.NewVolumeProvisioner(c.CloudProvider, c.ControllerManager.VolumeConfigFlags)
	if err != nil {
		// a provisioner was expected but encountered an error
		glog.Fatal(err)
	}

	// not all cloud providers have a provisioner.
	if provisioner != nil {
		allPlugins := []volume.VolumePlugin{}
		allPlugins = append(allPlugins, aws_ebs.ProbeVolumePlugins()...)
		allPlugins = append(allPlugins, gce_pd.ProbeVolumePlugins()...)
		allPlugins = append(allPlugins, cinder.ProbeVolumePlugins()...)
		controllerClient := volumeclaimbinder.NewControllerClient(client)
		provisionerController, err := volumeclaimbinder.NewPersistentVolumeProvisionerController(controllerClient, c.ControllerManager.PVClaimBinderSyncPeriod, allPlugins, provisioner, c.CloudProvider)
		if err != nil {
			glog.Fatalf("Could not start Persistent Volume Provisioner: %+v", err)
		}
		provisionerController.Run()
	}
}

func (c *MasterConfig) RunPersistentVolumeClaimRecycler(recyclerImageName string, client *client.Client) {
	uid := int64(0)
	defaultScrubPod := volume.NewPersistentVolumeRecyclerPodTemplate()
	defaultScrubPod.Spec.Containers[0].Image = recyclerImageName
	defaultScrubPod.Spec.Containers[0].Command = []string{"/usr/bin/recycle"}
	defaultScrubPod.Spec.Containers[0].Args = []string{"/scrub"}
	defaultScrubPod.Spec.Containers[0].SecurityContext = &kapi.SecurityContext{RunAsUser: &uid}
	defaultScrubPod.Spec.Containers[0].ImagePullPolicy = kapi.PullIfNotPresent

	volumeConfig := c.ControllerManager.VolumeConfigFlags
	hostPathConfig := volume.VolumeConfig{
		RecyclerMinimumTimeout:   volumeConfig.PersistentVolumeRecyclerMinimumTimeoutHostPath,
		RecyclerTimeoutIncrement: volumeConfig.PersistentVolumeRecyclerIncrementTimeoutHostPath,
		RecyclerPodTemplate:      defaultScrubPod,
	}

	if len(volumeConfig.PersistentVolumeRecyclerPodTemplateFilePathHostPath) != 0 {
		if err := attemptToLoadRecycler(volumeConfig.PersistentVolumeRecyclerPodTemplateFilePathHostPath, &hostPathConfig); err != nil {
			glog.Fatalf("Could not create hostpath recycler pod from file %s: %+v", volumeConfig.PersistentVolumeRecyclerPodTemplateFilePathHostPath, err)
		}
	}
	nfsConfig := volume.VolumeConfig{
		RecyclerMinimumTimeout:   volumeConfig.PersistentVolumeRecyclerMinimumTimeoutNFS,
		RecyclerTimeoutIncrement: volumeConfig.PersistentVolumeRecyclerIncrementTimeoutNFS,
		RecyclerPodTemplate:      defaultScrubPod,
	}

	if len(volumeConfig.PersistentVolumeRecyclerPodTemplateFilePathNFS) != 0 {
		if err := attemptToLoadRecycler(volumeConfig.PersistentVolumeRecyclerPodTemplateFilePathNFS, &nfsConfig); err != nil {
			glog.Fatalf("Could not create NFS recycler pod from file %s: %+v", volumeConfig.PersistentVolumeRecyclerPodTemplateFilePathNFS, err)
		}
	}

	allPlugins := []volume.VolumePlugin{}
	allPlugins = append(allPlugins, host_path.ProbeVolumePlugins(hostPathConfig)...)
	allPlugins = append(allPlugins, nfs.ProbeVolumePlugins(nfsConfig)...)

	// dynamic provisioning allows deletion of volumes as a recycling operation after a claim is deleted
	allPlugins = append(allPlugins, aws_ebs.ProbeVolumePlugins()...)
	allPlugins = append(allPlugins, gce_pd.ProbeVolumePlugins()...)
	allPlugins = append(allPlugins, cinder.ProbeVolumePlugins()...)

	recycler, err := volumeclaimbinder.NewPersistentVolumeRecycler(client, c.ControllerManager.PVClaimBinderSyncPeriod, allPlugins, c.CloudProvider)
	if err != nil {
		glog.Fatalf("Could not start Persistent Volume Recycler: %+v", err)
	}
	recycler.Run()
}

// attemptToLoadRecycler tries decoding a pod from a filepath for use as a recycler for a volume.
// If a path is not set as a CLI flag, no load will be attempted and no error returned.
// If a path is set and the pod was successfully loaded, the recycler pod will be set on the config and no error returned.
// Any failed attempt to load the recycler pod will return an error.
// TODO: make this func re-usable upstream and use downstream.  No need to duplicate this function.
func attemptToLoadRecycler(path string, config *volume.VolumeConfig) error {
	glog.V(5).Infof("Attempting to load recycler pod file from %s", path)
	recyclerPod, err := io.LoadPodFromFile(path)
	if err != nil {
		return err
	}
	if len(recyclerPod.Spec.Volumes) != 1 {
		return fmt.Errorf("Recycler pod is expected to have exactly 1 volume to scrub, but found %d", len(recyclerPod.Spec.Volumes))
	}
	config.RecyclerPodTemplate = recyclerPod
	glog.V(5).Infof("Recycler set to %s/%s", config.RecyclerPodTemplate.Namespace, config.RecyclerPodTemplate.Name)
	return nil
}

// RunReplicationController starts the Kubernetes replication controller sync loop
func (c *MasterConfig) RunReplicationController(client *client.Client) {
	controllerManager := replicationcontroller.NewReplicationManager(client, c.ControllerManager.ResyncPeriod, replicationcontroller.BurstReplicas)
	go controllerManager.Run(c.ControllerManager.ConcurrentRCSyncs, util.NeverStop)
}

// RunJobController starts the Kubernetes job controller sync loop
func (c *MasterConfig) RunJobController(client *client.Client) {
	controller := jobcontroller.NewJobController(client, c.ControllerManager.ResyncPeriod)
	go controller.Run(c.ControllerManager.ConcurrentJobSyncs, util.NeverStop)
}

// RunHPAController starts the Kubernetes hpa controller sync loop
func (c *MasterConfig) RunHPAController(oc *osclient.Client, kc *client.Client, heapsterNamespace string) {
	delegScaleNamespacer := osclient.NewDelegatingScaleNamespacer(oc, kc)
	podautoscaler := podautoscalercontroller.NewHorizontalController(kc, delegScaleNamespacer, kc, metrics.NewHeapsterMetricsClient(kc, heapsterNamespace, "https", "heapster", ""))
	podautoscaler.Run(c.ControllerManager.HorizontalPodAutoscalerSyncPeriod)
}

// RunEndpointController starts the Kubernetes replication controller sync loop
func (c *MasterConfig) RunEndpointController() {
	endpoints := endpointcontroller.NewEndpointController(c.KubeClient, c.ControllerManager.ResyncPeriod)
	go endpoints.Run(c.ControllerManager.ConcurrentEndpointSyncs, util.NeverStop)

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

		util.NewTokenBucketRateLimiter(s.DeletingPodsQps, s.DeletingPodsBurst),
		util.NewTokenBucketRateLimiter(s.DeletingPodsQps, s.DeletingPodsBurst), // upstream uses the same ones too

		s.NodeMonitorGracePeriod,
		s.NodeStartupGracePeriod,
		s.NodeMonitorPeriod,

		(*net.IPNet)(&s.ClusterCIDR),
		s.AllocateNodeCIDRs,
	)

	controller.Run(s.NodeSyncPeriod)
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
