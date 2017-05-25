package master

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"

	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/runtime"
	utilwait "k8s.io/apimachinery/pkg/util/wait"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	kv1core "k8s.io/client-go/kubernetes/typed/core/v1"
	kclientv1 "k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/tools/record"
	kctrlmgr "k8s.io/kubernetes/cmd/kube-controller-manager/app"
	kapi "k8s.io/kubernetes/pkg/api"
	kapiv1 "k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/apis/componentconfig"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/clientset"
	nodecontroller "k8s.io/kubernetes/pkg/controller/node"
	servicecontroller "k8s.io/kubernetes/pkg/controller/service"
	attachdetachcontroller "k8s.io/kubernetes/pkg/controller/volume/attachdetach"
	persistentvolumecontroller "k8s.io/kubernetes/pkg/controller/volume/persistentvolume"
	"k8s.io/kubernetes/pkg/features"
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
)

// this file contains our special cased controller initialization functions.
// TODO refactor this into the same controller init function style.  I suspect this means having the MasterConfig
// produce a set of controller init functions.  For now, don't mess with this to keep the diff sane.

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

func (c *MasterConfig) createSchedulerConfig() (*scheduler.Config, error) {
	var policy schedulerapi.Policy
	var configData []byte

	// TODO make the rate limiter configurable
	configFactory := factory.NewConfigFactory(
		c.SchedulerServer.SchedulerName,
		c.KubeClient,
		c.Informers.KubernetesInformers().Core().V1().Nodes(),
		c.Informers.KubernetesInformers().Core().V1().Pods(),
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
