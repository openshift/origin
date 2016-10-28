package kubernetes

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"time"

	"github.com/emicklei/go-restful"
	"github.com/golang/glog"

	kctrlmgr "k8s.io/kubernetes/cmd/kube-controller-manager/app"
	federationv1beta1 "k8s.io/kubernetes/federation/apis/federation/v1beta1"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/api/v1"
	appsv1alpha1 "k8s.io/kubernetes/pkg/apis/apps/v1alpha1"
	autoscalingv1 "k8s.io/kubernetes/pkg/apis/autoscaling/v1"
	"k8s.io/kubernetes/pkg/apis/batch"
	batchv1 "k8s.io/kubernetes/pkg/apis/batch/v1"
	batchv2alpha1 "k8s.io/kubernetes/pkg/apis/batch/v2alpha1"
	certificatesv1alpha1 "k8s.io/kubernetes/pkg/apis/certificates/v1alpha1"
	"k8s.io/kubernetes/pkg/apis/componentconfig"
	extv1beta1 "k8s.io/kubernetes/pkg/apis/extensions/v1beta1"
	policyv1alpha1 "k8s.io/kubernetes/pkg/apis/policy/v1alpha1"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/client/record"
	"k8s.io/kubernetes/pkg/client/restclient"
	"k8s.io/kubernetes/pkg/client/typed/dynamic"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	clientadapter "k8s.io/kubernetes/pkg/client/unversioned/adapters/internalclientset"
	"k8s.io/kubernetes/pkg/controller/deployment"
	"k8s.io/kubernetes/pkg/controller/garbagecollector/metaonly"
	"k8s.io/kubernetes/pkg/master"
	"k8s.io/kubernetes/pkg/registry/generic"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/runtime/serializer"
	"k8s.io/kubernetes/pkg/storage"
	storagefactory "k8s.io/kubernetes/pkg/storage/storagebackend/factory"
	utilwait "k8s.io/kubernetes/pkg/util/wait"

	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/controller/daemon"
	"k8s.io/kubernetes/pkg/controller/disruption"
	endpointcontroller "k8s.io/kubernetes/pkg/controller/endpoint"
	"k8s.io/kubernetes/pkg/controller/garbagecollector"
	jobcontroller "k8s.io/kubernetes/pkg/controller/job"
	namespacecontroller "k8s.io/kubernetes/pkg/controller/namespace"
	nodecontroller "k8s.io/kubernetes/pkg/controller/node"
	petsetcontroller "k8s.io/kubernetes/pkg/controller/petset"
	podautoscalercontroller "k8s.io/kubernetes/pkg/controller/podautoscaler"
	"k8s.io/kubernetes/pkg/controller/podautoscaler/metrics"
	gccontroller "k8s.io/kubernetes/pkg/controller/podgc"
	replicasetcontroller "k8s.io/kubernetes/pkg/controller/replicaset"
	replicationcontroller "k8s.io/kubernetes/pkg/controller/replication"
	"k8s.io/kubernetes/pkg/controller/scheduledjob"
	servicecontroller "k8s.io/kubernetes/pkg/controller/service"
	attachdetachcontroller "k8s.io/kubernetes/pkg/controller/volume/attachdetach"
	persistentvolumecontroller "k8s.io/kubernetes/pkg/controller/volume/persistentvolume"

	"k8s.io/kubernetes/pkg/registry/endpoint"
	endpointsetcd "k8s.io/kubernetes/pkg/registry/endpoint/etcd"
	"k8s.io/kubernetes/pkg/volume"
	"k8s.io/kubernetes/pkg/volume/aws_ebs"
	"k8s.io/kubernetes/pkg/volume/cinder"
	"k8s.io/kubernetes/pkg/volume/flexvolume"
	"k8s.io/kubernetes/pkg/volume/gce_pd"
	"k8s.io/kubernetes/pkg/volume/glusterfs"
	"k8s.io/kubernetes/pkg/volume/host_path"
	"k8s.io/kubernetes/pkg/volume/nfs"
	"k8s.io/kubernetes/pkg/volume/rbd"
	"k8s.io/kubernetes/pkg/volume/vsphere_volume"

	"k8s.io/kubernetes/plugin/pkg/scheduler"
	_ "k8s.io/kubernetes/plugin/pkg/scheduler/algorithmprovider"
	schedulerapi "k8s.io/kubernetes/plugin/pkg/scheduler/api"
	latestschedulerapi "k8s.io/kubernetes/plugin/pkg/scheduler/api/latest"
	"k8s.io/kubernetes/plugin/pkg/scheduler/factory"

	osclient "github.com/openshift/origin/pkg/client"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/election"
)

const (
	KubeAPIPrefix      = "/api"
	KubeAPIGroupPrefix = "/apis"
)

// InstallAPI starts a Kubernetes master and registers the supported REST APIs
// into the provided mux, then returns an array of strings indicating what
// endpoints were started (these are format strings that will expect to be sent
// a single string value).
func (c *MasterConfig) InstallAPI(container *restful.Container) ([]string, error) {
	c.Master.RestfulContainer = container

	if c.Master.EnableCoreControllers {
		glog.V(2).Info("Using the lease endpoint reconciler")
		config, err := c.Master.StorageFactory.NewConfig(kapi.Resource("apiServerIPInfo"))
		if err != nil {
			return nil, err
		}
		leaseStorage, _, err := storagefactory.Create(*config)
		if err != nil {
			return nil, err
		}
		masterLeases := newMasterLeases(leaseStorage)

		endpointConfig, err := c.Master.StorageFactory.NewConfig(kapi.Resource("endpoints"))
		if err != nil {
			return nil, err
		}
		endpointsStorage := endpointsetcd.NewREST(generic.RESTOptions{
			StorageConfig:           endpointConfig,
			Decorator:               generic.UndecoratedStorage,
			DeleteCollectionWorkers: 0,
			ResourcePrefix:          c.Master.StorageFactory.ResourcePrefix(kapi.Resource("endpoints")),
		})

		endpointRegistry := endpoint.NewRegistry(endpointsStorage)

		c.Master.EndpointReconcilerConfig = master.EndpointReconcilerConfig{
			Reconciler: election.NewLeaseEndpointReconciler(endpointRegistry, masterLeases),
			Interval:   master.DefaultEndpointReconcilerInterval,
		}
	}

	_, err := master.New(c.Master)
	if err != nil {
		return nil, err
	}

	messages := []string{}
	// v1 has to be printed separately since it's served from different endpoint than groups
	if configapi.HasKubernetesAPIVersion(c.Options, v1.SchemeGroupVersion) {
		messages = append(messages, fmt.Sprintf("Started Kubernetes API at %%s%s", KubeAPIPrefix))
	}

	// TODO: this is a bit much - I exist in some code somewhere
	versions := []unversioned.GroupVersion{
		extv1beta1.SchemeGroupVersion,
		batchv1.SchemeGroupVersion,
		batchv2alpha1.SchemeGroupVersion,
		autoscalingv1.SchemeGroupVersion,
		certificatesv1alpha1.SchemeGroupVersion,
		appsv1alpha1.SchemeGroupVersion,
		policyv1alpha1.SchemeGroupVersion,
		federationv1beta1.SchemeGroupVersion,
	}
	for _, ver := range versions {
		if configapi.HasKubernetesAPIVersion(c.Options, ver) {
			messages = append(messages, fmt.Sprintf("Started Kubernetes API %s at %%s%s", ver.String(), KubeAPIGroupPrefix))
		}
	}

	return messages, nil
}

func newMasterLeases(storage storage.Interface) election.Leases {
	// leaseTTL is in seconds, i.e. 15 means 15 seconds; do NOT do 15*time.Second!
	leaseTTL := uint64((master.DefaultEndpointReconcilerInterval + 5*time.Second) / time.Second) // add 5 seconds for wiggle room
	return election.NewLeases(storage, "/masterleases/", leaseTTL)
}

// RunNamespaceController starts the Kubernetes Namespace Manager
func (c *MasterConfig) RunNamespaceController(kubeClient internalclientset.Interface, clientPool dynamic.ClientPool) {
	// Find the list of namespaced resources via discovery that the namespace controller must manage
	groupVersionResources, err := kubeClient.Discovery().ServerPreferredNamespacedResources()
	if err != nil {
		glog.Fatalf("Failed to get supported resources from server: %v", err)
	}
	namespaceController := namespacecontroller.NewNamespaceController(kubeClient, clientPool, groupVersionResources, c.ControllerManager.NamespaceSyncPeriod.Duration, kapi.FinalizerKubernetes)
	go namespaceController.Run(int(c.ControllerManager.ConcurrentNamespaceSyncs), utilwait.NeverStop)
}

func (c *MasterConfig) RunPersistentVolumeController(client *client.Client, namespace, recyclerImageName, recyclerServiceAccountName string) {
	s := c.ControllerManager

	alphaProvisioner, err := kctrlmgr.NewAlphaVolumeProvisioner(c.CloudProvider, s.VolumeConfiguration)
	if err != nil {
		glog.Fatalf("A backward-compatible provisioner could not be created: %v, but one was expected. Provisioning will not work. This functionality is considered an early Alpha version.", err)
	}

	volumeController := persistentvolumecontroller.NewPersistentVolumeController(
		clientadapter.FromUnversionedClient(client),
		s.PVClaimBinderSyncPeriod.Duration,
		alphaProvisioner,
		probeRecyclableVolumePlugins(s.VolumeConfiguration, namespace, recyclerImageName, recyclerServiceAccountName),
		c.CloudProvider,
		s.ClusterName,
		nil, nil, nil,
		nil, // event recorder
		s.VolumeConfiguration.EnableDynamicProvisioning,
	)
	volumeController.Run(utilwait.NeverStop)
}

func (c *MasterConfig) RunPersistentVolumeAttachDetachController(client *client.Client) {
	s := c.ControllerManager
	attachDetachController, err :=
		attachdetachcontroller.NewAttachDetachController(
			clientadapter.FromUnversionedClient(client),
			c.Informers.Pods().Informer(),
			c.Informers.Nodes().Informer(),
			c.Informers.PersistentVolumeClaims().Informer(),
			c.Informers.PersistentVolumes().Informer(),
			c.CloudProvider,
			kctrlmgr.ProbeAttachableVolumePlugins(s.VolumeConfiguration),
			nil,
		)
	if err != nil {
		glog.Fatalf("Failed to start attach/detach controller: %v", err)
	} else {
		go attachDetachController.Run(utilwait.NeverStop)
	}
}

// probeRecyclableVolumePlugins collects all persistent volume plugins into an easy to use list.
func probeRecyclableVolumePlugins(config componentconfig.VolumeConfiguration, namespace, recyclerImageName, recyclerServiceAccountName string) []volume.VolumePlugin {
	uid := int64(0)
	defaultScrubPod := volume.NewPersistentVolumeRecyclerPodTemplate()
	defaultScrubPod.Namespace = namespace
	defaultScrubPod.Spec.ServiceAccountName = recyclerServiceAccountName
	defaultScrubPod.Spec.Containers[0].Image = recyclerImageName
	defaultScrubPod.Spec.Containers[0].Command = []string{"/usr/bin/openshift-recycle"}
	defaultScrubPod.Spec.Containers[0].Args = []string{"/scrub"}
	defaultScrubPod.Spec.Containers[0].SecurityContext = &kapi.SecurityContext{RunAsUser: &uid}
	defaultScrubPod.Spec.Containers[0].ImagePullPolicy = kapi.PullIfNotPresent

	allPlugins := []volume.VolumePlugin{}

	// The list of plugins to probe is decided by this binary, not
	// by dynamic linking or other "magic".  Plugins will be analyzed and
	// initialized later.

	// Each plugin can make use of VolumeConfig.  The single arg to this func contains *all* enumerated
	// options meant to configure volume plugins.  From that single config, create an instance of volume.VolumeConfig
	// for a specific plugin and pass that instance to the plugin's ProbeVolumePlugins(config) func.

	// HostPath recycling is for testing and development purposes only!
	hostPathConfig := volume.VolumeConfig{
		RecyclerMinimumTimeout:   int(config.PersistentVolumeRecyclerConfiguration.MinimumTimeoutHostPath),
		RecyclerTimeoutIncrement: int(config.PersistentVolumeRecyclerConfiguration.IncrementTimeoutHostPath),
		RecyclerPodTemplate:      defaultScrubPod,
	}
	if err := kctrlmgr.AttemptToLoadRecycler(config.PersistentVolumeRecyclerConfiguration.PodTemplateFilePathHostPath, &hostPathConfig); err != nil {
		glog.Fatalf("Could not create hostpath recycler pod from file %s: %+v", config.PersistentVolumeRecyclerConfiguration.PodTemplateFilePathHostPath, err)
	}
	allPlugins = append(allPlugins, host_path.ProbeVolumePlugins(hostPathConfig)...)

	nfsConfig := volume.VolumeConfig{
		RecyclerMinimumTimeout:   int(config.PersistentVolumeRecyclerConfiguration.MinimumTimeoutNFS),
		RecyclerTimeoutIncrement: int(config.PersistentVolumeRecyclerConfiguration.IncrementTimeoutNFS),
		RecyclerPodTemplate:      defaultScrubPod,
	}
	if err := kctrlmgr.AttemptToLoadRecycler(config.PersistentVolumeRecyclerConfiguration.PodTemplateFilePathNFS, &nfsConfig); err != nil {
		glog.Fatalf("Could not create NFS recycler pod from file %s: %+v", config.PersistentVolumeRecyclerConfiguration.PodTemplateFilePathNFS, err)
	}
	allPlugins = append(allPlugins, nfs.ProbeVolumePlugins(nfsConfig)...)

	allPlugins = append(allPlugins, aws_ebs.ProbeVolumePlugins()...)
	allPlugins = append(allPlugins, gce_pd.ProbeVolumePlugins()...)
	allPlugins = append(allPlugins, cinder.ProbeVolumePlugins()...)
	allPlugins = append(allPlugins, flexvolume.ProbeVolumePlugins(config.FlexVolumePluginDir)...)
	allPlugins = append(allPlugins, vsphere_volume.ProbeVolumePlugins()...)
	allPlugins = append(allPlugins, glusterfs.ProbeVolumePlugins()...)
	allPlugins = append(allPlugins, rbd.ProbeVolumePlugins()...)

	return allPlugins
}

func (c *MasterConfig) RunReplicaSetController(client *client.Client) {
	controller := replicasetcontroller.NewReplicaSetController(
		c.Informers.Pods().Informer(),
		clientadapter.FromUnversionedClient(client),
		kctrlmgr.ResyncPeriod(c.ControllerManager),
		replicasetcontroller.BurstReplicas,
		int(c.ControllerManager.LookupCacheSizeForRC),
		c.ControllerManager.EnableGarbageCollector,
	)
	go controller.Run(int(c.ControllerManager.ConcurrentRSSyncs), utilwait.NeverStop)
}

// RunReplicationController starts the Kubernetes replication controller sync loop
func (c *MasterConfig) RunReplicationController(client *client.Client) {
	controllerManager := replicationcontroller.NewReplicationManager(
		c.Informers.Pods().Informer(),
		clientadapter.FromUnversionedClient(client),
		kctrlmgr.ResyncPeriod(c.ControllerManager),
		replicationcontroller.BurstReplicas,
		int(c.ControllerManager.LookupCacheSizeForRC),
		c.ControllerManager.EnableGarbageCollector,
	)
	go controllerManager.Run(int(c.ControllerManager.ConcurrentRCSyncs), utilwait.NeverStop)
}

func (c *MasterConfig) RunDeploymentController(client *client.Client) {
	controller := deployment.NewDeploymentController(
		clientadapter.FromUnversionedClient(client),
		kctrlmgr.ResyncPeriod(c.ControllerManager),
	)
	go controller.Run(int(c.ControllerManager.ConcurrentDeploymentSyncs), utilwait.NeverStop)
}

// RunJobController starts the Kubernetes job controller sync loop
func (c *MasterConfig) RunJobController(client *client.Client) {
	controller := jobcontroller.NewJobController(c.Informers.Pods().Informer(), clientadapter.FromUnversionedClient(client))
	go controller.Run(int(c.ControllerManager.ConcurrentJobSyncs), utilwait.NeverStop)
}

// RunScheduledJobController starts the Kubernetes scheduled job controller sync loop
func (c *MasterConfig) RunScheduledJobController(config *restclient.Config) {
	// TODO: this is a temp fix for allowing kubeClient list v2alpha1 jobs, should switch to using clientset
	config.ContentConfig.GroupVersion = &unversioned.GroupVersion{Group: batch.GroupName, Version: "v2alpha1"}
	client, err := kclient.New(config)
	if err != nil {
		glog.Fatalf("Unable to configure scheduled job controller: %v", err)
	}
	go scheduledjob.NewScheduledJobController(client).Run(utilwait.NeverStop)
}

// RunDisruptionBudgetController starts the Kubernetes disruption budget controller
func (c *MasterConfig) RunDisruptionBudgetController(client *client.Client) {
	go disruption.NewDisruptionController(c.Informers.Pods().Informer(), client).Run(utilwait.NeverStop)
}

// RunHPAController starts the Kubernetes hpa controller sync loop
func (c *MasterConfig) RunHPAController(oc *osclient.Client, kc *client.Client, heapsterNamespace string) {
	clientsetClient := clientadapter.FromUnversionedClient(kc)
	delegatingScaleNamespacer := osclient.NewDelegatingScaleNamespacer(oc, kc)
	podautoscaler := podautoscalercontroller.NewHorizontalController(
		clientsetClient,
		delegatingScaleNamespacer,
		clientsetClient,
		metrics.NewHeapsterMetricsClient(clientsetClient, heapsterNamespace, "https", "heapster", ""),
		c.ControllerManager.HorizontalPodAutoscalerSyncPeriod.Duration,
	)
	go podautoscaler.Run(utilwait.NeverStop)
}

func (c *MasterConfig) RunDaemonSetsController(client *client.Client) {
	controller := daemon.NewDaemonSetsController(
		c.Informers.Pods().Informer(),
		clientadapter.FromUnversionedClient(client),
		kctrlmgr.ResyncPeriod(c.ControllerManager),
		int(c.ControllerManager.LookupCacheSizeForDaemonSet),
	)
	go controller.Run(int(c.ControllerManager.ConcurrentDaemonSetSyncs), utilwait.NeverStop)
}

// RunEndpointController starts the Kubernetes replication controller sync loop
func (c *MasterConfig) RunEndpointController(client *client.Client) {
	endpoints := endpointcontroller.NewEndpointController(c.Informers.Pods().Informer(), clientadapter.FromUnversionedClient(client))
	go endpoints.Run(int(c.ControllerManager.ConcurrentEndpointSyncs), utilwait.NeverStop)

}

// RunScheduler starts the Kubernetes scheduler
func (c *MasterConfig) RunScheduler() {
	config, err := c.createSchedulerConfig()
	if err != nil {
		glog.Fatalf("Unable to start scheduler: %v", err)
	}
	eventcast := record.NewBroadcaster()
	config.Recorder = eventcast.NewRecorder(kapi.EventSource{Component: kapi.DefaultSchedulerName})
	eventcast.StartRecordingToSink(c.KubeClient.Events(""))

	s := scheduler.New(config)
	s.Run()
}

// RunGCController handles deletion of terminated pods.
func (c *MasterConfig) RunGCController(client *client.Client) {
	if c.ControllerManager.TerminatedPodGCThreshold > 0 {
		gcController := gccontroller.New(clientadapter.FromUnversionedClient(client), kctrlmgr.ResyncPeriod(c.ControllerManager), int(c.ControllerManager.TerminatedPodGCThreshold))
		go gcController.Run(utilwait.NeverStop)
	}
}

// RunGarbageCollectorController starts generic garbage collection for the cluster.
func (c *MasterConfig) RunGarbageCollectorController(client *osclient.Client, config *restclient.Config) {
	if !c.ControllerManager.EnableGarbageCollector {
		return
	}

	groupVersionResources, err := client.Discovery().ServerPreferredResources()
	if err != nil {
		glog.Fatalf("Failed to get supported resources from server: %v", err)
	}

	config = restclient.AddUserAgent(config, "generic-garbage-collector")
	config.ContentConfig.NegotiatedSerializer = serializer.DirectCodecFactory{CodecFactory: metaonly.NewMetadataCodecFactory()}
	// TODO: needs to take GVR
	metaOnlyClientPool := dynamic.NewClientPool(config, dynamic.LegacyAPIPathResolverFunc)
	config.ContentConfig.NegotiatedSerializer = nil
	// TODO: needs to take GVR
	clientPool := dynamic.NewClientPool(config, dynamic.LegacyAPIPathResolverFunc)
	garbageCollector, err := garbagecollector.NewGarbageCollector(metaOnlyClientPool, clientPool, groupVersionResources)
	if err != nil {
		glog.Fatalf("Failed to start the garbage collector: %v", err)
	}

	workers := int(c.ControllerManager.ConcurrentGCSyncs)
	go garbageCollector.Run(workers, utilwait.NeverStop)
}

// RunNodeController starts the node controller
// TODO: handle node CIDR and route allocation
func (c *MasterConfig) RunNodeController() {
	s := c.ControllerManager

	// this cidr has been validated already
	_, clusterCIDR, _ := net.ParseCIDR(s.ClusterCIDR)
	_, serviceCIDR, _ := net.ParseCIDR(s.ServiceCIDR)

	controller, err := nodecontroller.NewNodeController(
		c.Informers.Pods().Informer(),
		c.CloudProvider,
		clientadapter.FromUnversionedClient(c.KubeClient),
		s.PodEvictionTimeout.Duration,

		s.NodeEvictionRate,
		s.SecondaryNodeEvictionRate,
		s.LargeClusterSizeThreshold,
		s.UnhealthyZoneThreshold,

		s.NodeMonitorGracePeriod.Duration,
		s.NodeStartupGracePeriod.Duration,
		s.NodeMonitorPeriod.Duration,

		clusterCIDR,
		serviceCIDR,

		int(s.NodeCIDRMaskSize),
		s.AllocateNodeCIDRs,
	)
	if err != nil {
		glog.Fatalf("Unable to start node controller: %v", err)
	}

	controller.Run()
}

// RunServiceLoadBalancerController starts the service loadbalancer controller if the cloud provider is configured.
func (c *MasterConfig) RunServiceLoadBalancerController(client *client.Client) {
	if c.CloudProvider == nil {
		glog.V(2).Infof("Service controller will not start - no cloud provider configured")
		return
	}
	serviceController, err := servicecontroller.New(c.CloudProvider, clientadapter.FromUnversionedClient(client), c.ControllerManager.ClusterName)
	if err != nil {
		glog.Errorf("Unable to start service controller: %v", err)
	} else {
		serviceController.Run(int(c.ControllerManager.ConcurrentServiceSyncs))
	}
}

// RunPetSetController starts the PetSet controller
func (c *MasterConfig) RunPetSetController(client *client.Client) {
	ps := petsetcontroller.NewPetSetController(c.Informers.Pods().Informer(), client, kctrlmgr.ResyncPeriod(c.ControllerManager)())
	go ps.Run(1, utilwait.NeverStop)
}

func (c *MasterConfig) createSchedulerConfig() (*scheduler.Config, error) {
	var policy schedulerapi.Policy
	var configData []byte

	// TODO make the rate limiter configurable
	configFactory := factory.NewConfigFactory(c.KubeClient, c.SchedulerServer.SchedulerName, int(c.SchedulerServer.HardPodAffinitySymmetricWeight), c.SchedulerServer.FailureDomains)
	if _, err := os.Stat(c.Options.SchedulerConfigFile); err == nil {
		configData, err = ioutil.ReadFile(c.SchedulerServer.PolicyConfigFile)
		if err != nil {
			return nil, fmt.Errorf("unable to read scheduler config: %v", err)
		}
		err = runtime.DecodeInto(latestschedulerapi.Codec, configData, &policy)
		if err != nil {
			return nil, fmt.Errorf("invalid scheduler configuration: %v", err)
		}

		return configFactory.CreateFromConfig(policy)
	}

	// if the config file isn't provided, use the default provider
	return configFactory.CreateFromProvider(factory.DefaultProvider)
}
