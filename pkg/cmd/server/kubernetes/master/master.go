package master

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"time"

	"github.com/golang/glog"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilwait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/storage"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	kv1core "k8s.io/client-go/kubernetes/typed/core/v1"
	kclientv1 "k8s.io/client-go/pkg/api/v1"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	kctrlmgr "k8s.io/kubernetes/cmd/kube-controller-manager/app"
	kapi "k8s.io/kubernetes/pkg/api"
	kapiv1 "k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/apis/certificates"
	"k8s.io/kubernetes/pkg/apis/componentconfig"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/clientset"
	coreinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/externalversions/core/v1"
	certcontroller "k8s.io/kubernetes/pkg/controller/certificates"
	"k8s.io/kubernetes/pkg/controller/cronjob"
	"k8s.io/kubernetes/pkg/controller/daemon"
	"k8s.io/kubernetes/pkg/controller/deployment"
	"k8s.io/kubernetes/pkg/controller/disruption"
	endpointcontroller "k8s.io/kubernetes/pkg/controller/endpoint"
	"k8s.io/kubernetes/pkg/controller/garbagecollector"
	"k8s.io/kubernetes/pkg/controller/garbagecollector/metaonly"
	jobcontroller "k8s.io/kubernetes/pkg/controller/job"
	namespacecontroller "k8s.io/kubernetes/pkg/controller/namespace"
	nodecontroller "k8s.io/kubernetes/pkg/controller/node"
	podautoscalercontroller "k8s.io/kubernetes/pkg/controller/podautoscaler"
	"k8s.io/kubernetes/pkg/controller/podautoscaler/metrics"
	gccontroller "k8s.io/kubernetes/pkg/controller/podgc"
	replicasetcontroller "k8s.io/kubernetes/pkg/controller/replicaset"
	replicationcontroller "k8s.io/kubernetes/pkg/controller/replication"
	servicecontroller "k8s.io/kubernetes/pkg/controller/service"
	statefulsetcontroller "k8s.io/kubernetes/pkg/controller/statefulset"
	attachdetachcontroller "k8s.io/kubernetes/pkg/controller/volume/attachdetach"
	persistentvolumecontroller "k8s.io/kubernetes/pkg/controller/volume/persistentvolume"
	"k8s.io/kubernetes/pkg/features"
	"k8s.io/kubernetes/pkg/master"
	"k8s.io/kubernetes/pkg/volume"
	"k8s.io/kubernetes/pkg/volume/aws_ebs"
	"k8s.io/kubernetes/pkg/volume/azure_dd"
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
	"github.com/openshift/origin/pkg/cmd/server/election"
)

func newMasterLeases(storage storage.Interface) election.Leases {
	// leaseTTL is in seconds, i.e. 15 means 15 seconds; do NOT do 15*time.Second!
	leaseTTL := uint64((master.DefaultEndpointReconcilerInterval + 5*time.Second) / time.Second) // add 5 seconds for wiggle room
	return election.NewLeases(storage, "/masterleases/", leaseTTL)
}

// RunNamespaceController starts the Kubernetes Namespace Manager
func (c *MasterConfig) RunNamespaceController(kubeClient kclientset.Interface, clientPool dynamic.ClientPool, namespaceInformer coreinformers.NamespaceInformer) {
	// Find the list of namespaced resources via discovery that the namespace controller must manage
	groupVersionResources, err := kubeClient.Discovery().ServerPreferredNamespacedResources()
	if err != nil {
		glog.Fatalf("Failed to get resources: %v", err)
	}
	gvrFn := func() ([]*metav1.APIResourceList, error) {
		return groupVersionResources, nil
	}
	namespaceController := namespacecontroller.NewNamespaceController(kubeClient, clientPool, gvrFn, namespaceInformer, c.ControllerManager.NamespaceSyncPeriod.Duration, kapiv1.FinalizerKubernetes)
	go namespaceController.Run(int(c.ControllerManager.ConcurrentNamespaceSyncs), utilwait.NeverStop)
}

func (c *MasterConfig) RunPersistentVolumeController(client kclientset.Interface, namespace, recyclerImageName, recyclerServiceAccountName string) {
	s := c.ControllerManager

	alphaProvisioner, err := kctrlmgr.NewAlphaVolumeProvisioner(c.CloudProvider, s.VolumeConfiguration)
	if err != nil {
		glog.Fatalf("A backward-compatible provisioner could not be created: %v, but one was expected. Provisioning will not work. This functionality is considered an early Alpha version.", err)
	}

	eventcast := record.NewBroadcaster()
	recorder := eventcast.NewRecorder(kapi.Scheme, kclientv1.EventSource{Component: "persistent-volume-controller"})
	eventcast.StartRecordingToSink(&kv1core.EventSinkImpl{Interface: kv1core.New(c.KubeClient.CoreV1().RESTClient()).Events("")})

	volumeController := persistentvolumecontroller.NewController(
		persistentvolumecontroller.ControllerParameters{
			KubeClient:                client,
			SyncPeriod:                s.PVClaimBinderSyncPeriod.Duration,
			AlphaProvisioner:          alphaProvisioner,
			VolumePlugins:             probeRecyclableVolumePlugins(s.VolumeConfiguration, namespace, recyclerImageName, recyclerServiceAccountName),
			Cloud:                     c.CloudProvider,
			ClusterName:               s.ClusterName,
			VolumeInformer:            c.Informers.KubernetesInformers().Core().V1().PersistentVolumes(),
			ClaimInformer:             c.Informers.KubernetesInformers().Core().V1().PersistentVolumeClaims(),
			ClassInformer:             c.Informers.KubernetesInformers().Storage().V1beta1().StorageClasses(),
			EventRecorder:             recorder,
			EnableDynamicProvisioning: s.VolumeConfiguration.EnableDynamicProvisioning,
		})
	go volumeController.Run(utilwait.NeverStop)
}

func (c *MasterConfig) RunPersistentVolumeAttachDetachController(client kclientset.Interface) {
	s := c.ControllerManager
	attachDetachController, err :=
		attachdetachcontroller.NewAttachDetachController(
			client,
			c.Informers.KubernetesInformers().Core().V1().Pods(),
			c.Informers.KubernetesInformers().Core().V1().Nodes(),
			c.Informers.KubernetesInformers().Core().V1().PersistentVolumeClaims(),
			c.Informers.KubernetesInformers().Core().V1().PersistentVolumes(),
			c.CloudProvider,
			kctrlmgr.ProbeAttachableVolumePlugins(s.VolumeConfiguration),
			s.DisableAttachDetachReconcilerSync,
			s.ReconcilerSyncLoopPeriod.Duration,
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
	defaultScrubPod.Spec.Containers[0].SecurityContext = &kapiv1.SecurityContext{RunAsUser: &uid}
	defaultScrubPod.Spec.Containers[0].ImagePullPolicy = kapiv1.PullIfNotPresent

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
		ProvisioningEnabled:      config.EnableHostPathProvisioning,
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
	allPlugins = append(allPlugins, azure_dd.ProbeVolumePlugins()...)

	return allPlugins
}

func (c *MasterConfig) RunReplicaSetController(client kclientset.Interface) {
	controller := replicasetcontroller.NewReplicaSetController(
		c.Informers.KubernetesInformers().Extensions().V1beta1().ReplicaSets(),
		c.Informers.KubernetesInformers().Core().V1().Pods(),
		client,
		replicasetcontroller.BurstReplicas,
	)
	go controller.Run(int(c.ControllerManager.ConcurrentRSSyncs), utilwait.NeverStop)
}

// RunReplicationController starts the Kubernetes replication controller sync loop
func (c *MasterConfig) RunReplicationController(client kclientset.Interface) {
	controllerManager := replicationcontroller.NewReplicationManager(
		c.Informers.KubernetesInformers().Core().V1().Pods(),
		c.Informers.KubernetesInformers().Core().V1().ReplicationControllers(),
		client,
		replicationcontroller.BurstReplicas,
	)
	go controllerManager.Run(int(c.ControllerManager.ConcurrentRCSyncs), utilwait.NeverStop)
}

func (c *MasterConfig) RunDeploymentController(client kclientset.Interface) {
	controller := deployment.NewDeploymentController(
		c.Informers.KubernetesInformers().Extensions().V1beta1().Deployments(),
		c.Informers.KubernetesInformers().Extensions().V1beta1().ReplicaSets(),
		c.Informers.KubernetesInformers().Core().V1().Pods(),
		client,
	)
	go controller.Run(int(c.ControllerManager.ConcurrentDeploymentSyncs), utilwait.NeverStop)
}

// RunJobController starts the Kubernetes job controller sync loop
func (c *MasterConfig) RunJobController(client kclientset.Interface) {
	controller := jobcontroller.NewJobController(
		c.Informers.KubernetesInformers().Core().V1().Pods(),
		c.Informers.KubernetesInformers().Batch().V1().Jobs(),
		client,
	)
	go controller.Run(int(c.ControllerManager.ConcurrentJobSyncs), utilwait.NeverStop)
}

// RunCronJobController starts the Kubernetes scheduled job controller sync loop
func (c *MasterConfig) RunCronJobController(client kclientset.Interface) {
	go cronjob.NewCronJobController(client).Run(utilwait.NeverStop)
}

// RunDisruptionBudgetController starts the Kubernetes disruption budget controller
func (c *MasterConfig) RunDisruptionBudgetController(client kclientset.Interface) {
	go disruption.NewDisruptionController(
		c.Informers.KubernetesInformers().Core().V1().Pods(),
		c.Informers.KubernetesInformers().Policy().V1beta1().PodDisruptionBudgets(),
		c.Informers.KubernetesInformers().Core().V1().ReplicationControllers(),
		c.Informers.KubernetesInformers().Extensions().V1beta1().ReplicaSets(),
		c.Informers.KubernetesInformers().Extensions().V1beta1().Deployments(),
		c.Informers.KubernetesInformers().Apps().V1beta1().StatefulSets(),
		client,
	).Run(utilwait.NeverStop)
}

// RunHPAController starts the Kubernetes hpa controller sync loop
func (c *MasterConfig) RunHPAController(oc *osclient.Client, kc kclientset.Interface, heapsterNamespace string) {
	delegatingScaleNamespacer := osclient.NewDelegatingScaleNamespacer(oc, kc.ExtensionsV1beta1())
	metricsClient := metrics.NewHeapsterMetricsClient(kc, heapsterNamespace, "https", "heapster", "")
	replicaCalc := podautoscalercontroller.NewReplicaCalculator(metricsClient, kc.Core())
	podautoscaler := podautoscalercontroller.NewHorizontalController(
		kv1core.New(kc.Core().RESTClient()),
		delegatingScaleNamespacer,
		kc.AutoscalingV1(),
		replicaCalc,
		c.Informers.KubernetesInformers().Autoscaling().V1().HorizontalPodAutoscalers(),
		c.ControllerManager.HorizontalPodAutoscalerSyncPeriod.Duration,
	)
	go podautoscaler.Run(utilwait.NeverStop)
}

func (c *MasterConfig) RunDaemonSetsController(client kclientset.Interface) {
	controller := daemon.NewDaemonSetsController(
		c.Informers.KubernetesInformers().Extensions().V1beta1().DaemonSets(),
		c.Informers.KubernetesInformers().Core().V1().Pods(),
		c.Informers.KubernetesInformers().Core().V1().Nodes(),
		client,
	)
	go controller.Run(int(c.ControllerManager.ConcurrentDaemonSetSyncs), utilwait.NeverStop)
}

// RunEndpointController starts the Kubernetes replication controller sync loop
func (c *MasterConfig) RunEndpointController(client kclientset.Interface) {
	endpoints := endpointcontroller.NewEndpointController(
		c.Informers.KubernetesInformers().Core().V1().Pods(),
		c.Informers.KubernetesInformers().Core().V1().Services(),
		client,
	)
	go endpoints.Run(int(c.ControllerManager.ConcurrentEndpointSyncs), utilwait.NeverStop)
}

// RunScheduler starts the Kubernetes scheduler
func (c *MasterConfig) RunScheduler() {
	config, err := c.createSchedulerConfig()
	if err != nil {
		glog.Fatalf("Unable to start scheduler: %v", err)
	}
	eventcast := record.NewBroadcaster()
	config.Recorder = eventcast.NewRecorder(kapi.Scheme, kclientv1.EventSource{Component: kapi.DefaultSchedulerName})
	eventcast.StartRecordingToSink(&kv1core.EventSinkImpl{Interface: kv1core.New(c.KubeClient.CoreV1().RESTClient()).Events("")})

	s := scheduler.New(config)
	go s.Run()
}

// RunGCController handles deletion of terminated pods.
func (c *MasterConfig) RunGCController(client kclientset.Interface) {
	if c.ControllerManager.TerminatedPodGCThreshold > 0 {
		gcController := gccontroller.NewPodGC(
			client,
			c.Informers.KubernetesInformers().Core().V1().Pods(),
			int(c.ControllerManager.TerminatedPodGCThreshold),
		)
		go gcController.Run(utilwait.NeverStop)
	}
}

// RunGarbageCollectorController starts generic garbage collection for the cluster.
func (c *MasterConfig) RunGarbageCollectorController(client *osclient.Client, config *restclient.Config) {
	if !c.ControllerManager.EnableGarbageCollector {
		return
	}

	preferredResources, err := client.Discovery().ServerPreferredResources()
	if err != nil {
		glog.Fatalf("failed to get supported resources from server: %v", err)
	}

	deletableResources := discovery.FilteredBy(discovery.SupportsAllVerbs{Verbs: []string{"delete"}}, preferredResources)
	deletableGroupVersionResources, err := discovery.GroupVersionResources(deletableResources)
	if err != nil {
		glog.Fatalf("Failed to parse resources from server: %v", err)
	}

	config = restclient.AddUserAgent(config, "generic-garbage-collector")
	config.ContentConfig.NegotiatedSerializer = serializer.DirectCodecFactory{CodecFactory: metaonly.NewMetadataCodecFactory()}
	// TODO: should use a dynamic RESTMapper built from the discovery results.
	restMapper := kapi.Registry.RESTMapper()
	// TODO: needs to take GVR
	metaOnlyClientPool := dynamic.NewClientPool(config, restMapper, dynamic.LegacyAPIPathResolverFunc)
	config.ContentConfig.NegotiatedSerializer = nil
	// TODO: needs to take GVR
	clientPool := dynamic.NewClientPool(config, restMapper, dynamic.LegacyAPIPathResolverFunc)
	garbageCollector, err := garbagecollector.NewGarbageCollector(metaOnlyClientPool, clientPool, restMapper, deletableGroupVersionResources)
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
		c.Informers.KubernetesInformers().Core().V1().Pods(),
		c.Informers.KubernetesInformers().Core().V1().Nodes(),
		c.Informers.KubernetesInformers().Extensions().V1beta1().DaemonSets(),
		c.CloudProvider,
		c.KubeClient,
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
		s.EnableTaintManager,
		utilfeature.DefaultFeatureGate.Enabled(features.TaintBasedEvictions),
	)
	if err != nil {
		glog.Fatalf("Unable to start node controller: %v", err)
	}

	go controller.Run()
}

// RunServiceLoadBalancerController starts the service loadbalancer controller if the cloud provider is configured.
func (c *MasterConfig) RunServiceLoadBalancerController(client kclientset.Interface) {
	if c.CloudProvider == nil {
		glog.V(2).Infof("Service controller will not start - no cloud provider configured")
		return
	}
	serviceController, err := servicecontroller.New(
		c.CloudProvider,
		client,
		c.Informers.KubernetesInformers().Core().V1().Services(),
		c.Informers.KubernetesInformers().Core().V1().Nodes(),
		c.ControllerManager.ClusterName,
	)
	if err != nil {
		glog.Errorf("Unable to start service controller: %v", err)
	} else {
		go serviceController.Run(utilwait.NeverStop, int(c.ControllerManager.ConcurrentServiceSyncs))
	}
}

// RunStatefulSetController starts the StatefulSet controller
func (c *MasterConfig) RunStatefulSetController(client kclientset.Interface) {
	ps := statefulsetcontroller.NewStatefulSetController(
		c.Informers.KubernetesInformers().Core().V1().Pods(),
		c.Informers.KubernetesInformers().Apps().V1beta1().StatefulSets(),
		c.Informers.KubernetesInformers().Core().V1().PersistentVolumeClaims(),
		client,
	)
	go ps.Run(1, utilwait.NeverStop)
}

func (c *MasterConfig) createSchedulerConfig() (*scheduler.Config, error) {
	var policy schedulerapi.Policy
	var configData []byte

	// TODO make the rate limiter configurable
	configFactory := factory.NewConfigFactory(
		c.SchedulerServer.SchedulerName,
		c.KubeClient,
		c.Informers.KubernetesInformers().Core().V1().Nodes(),
		c.Informers.KubernetesInformers().Core().V1().PersistentVolumes(),
		c.Informers.KubernetesInformers().Core().V1().PersistentVolumeClaims(),
		c.Informers.KubernetesInformers().Core().V1().ReplicationControllers(),
		c.Informers.KubernetesInformers().Extensions().V1beta1().ReplicaSets(),
		c.Informers.KubernetesInformers().Apps().V1beta1().StatefulSets(),
		c.Informers.KubernetesInformers().Core().V1().Services(),
		int(c.SchedulerServer.HardPodAffinitySymmetricWeight),
	)
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

type noAutoApproval struct{}

func (noAutoApproval) AutoApprove(csr *certificates.CertificateSigningRequest) (*certificates.CertificateSigningRequest, error) {
	return csr, nil
}

func (c *MasterConfig) RunCertificateSigningController(clientset kclientset.Interface) {
	if len(c.ControllerManager.ClusterSigningCertFile) == 0 || len(c.ControllerManager.ClusterSigningKeyFile) == 0 {
		glog.V(2).Infof("Certificate signer controller will not start - no signing key or cert set")
		return
	}

	signer, err := certcontroller.NewCFSSLSigner(c.ControllerManager.ClusterSigningCertFile, c.ControllerManager.ClusterSigningKeyFile)
	if err != nil {
		glog.Fatalf("Failed to start certificate controller: %v", err)
	}

	certController, err := certcontroller.NewCertificateController(
		clientset,
		c.Informers.KubernetesInformers().Certificates().V1beta1().CertificateSigningRequests(),
		signer,
		certcontroller.NewGroupApprover(c.ControllerManager.ApproveAllKubeletCSRsForGroup),
	)
	if err != nil {
		glog.Fatalf("Failed to start certificate controller: %v", err)
	}
	go certController.Run(1, utilwait.NeverStop)
}
