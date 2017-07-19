package controller

import (
	"fmt"

	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"

	kv1core "k8s.io/client-go/kubernetes/typed/core/v1"
	kclientv1 "k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/tools/record"
	kctrlmgr "k8s.io/kubernetes/cmd/kube-controller-manager/app"
	kubecontroller "k8s.io/kubernetes/cmd/kube-controller-manager/app"
	kapi "k8s.io/kubernetes/pkg/api"
	kapiv1 "k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/apis/componentconfig"
	persistentvolumecontroller "k8s.io/kubernetes/pkg/controller/volume/persistentvolume"
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
)

type PersistentVolumeControllerConfig struct {
	RecyclerImage string
}

func (c *PersistentVolumeControllerConfig) RunController(ctx kubecontroller.ControllerContext) (bool, error) {
	eventcast := record.NewBroadcaster()
	recorder := eventcast.NewRecorder(kapi.Scheme, kclientv1.EventSource{Component: "persistentvolume-controller"})
	eventcast.StartRecordingToSink(
		&kv1core.EventSinkImpl{
			Interface: ctx.ClientBuilder.ClientGoClientOrDie("persistent-volume-binder").CoreV1().Events(""),
		},
	)

	plugins, err := probeRecyclableVolumePlugins(ctx.Options.VolumeConfiguration, c.RecyclerImage, bootstrappolicy.InfraPersistentVolumeRecyclerControllerServiceAccountName)
	if err != nil {
		return false, err
	}

	volumeController, volumeControllerErr := persistentvolumecontroller.NewController(
		persistentvolumecontroller.ControllerParameters{
			KubeClient:                ctx.ClientBuilder.ClientOrDie("persistent-volume-binder"),
			SyncPeriod:                ctx.Options.PVClaimBinderSyncPeriod.Duration,
			VolumePlugins:             plugins,
			Cloud:                     ctx.Cloud,
			ClusterName:               ctx.Options.ClusterName,
			VolumeInformer:            ctx.InformerFactory.Core().V1().PersistentVolumes(),
			ClaimInformer:             ctx.InformerFactory.Core().V1().PersistentVolumeClaims(),
			ClassInformer:             ctx.InformerFactory.Storage().V1().StorageClasses(),
			EventRecorder:             recorder,
			EnableDynamicProvisioning: ctx.Options.VolumeConfiguration.EnableDynamicProvisioning,
		})
	if volumeControllerErr != nil {
		return true, fmt.Errorf("failed to construct persistentvolume controller: %v", volumeControllerErr)
	}
	go volumeController.Run(ctx.Stop)

	return true, nil
}

// probeRecyclableVolumePlugins collects all persistent volume plugins into an easy to use list.
// TODO: Move this into some helper package?
func probeRecyclableVolumePlugins(config componentconfig.VolumeConfiguration, recyclerImageName, recyclerServiceAccountName string) ([]volume.VolumePlugin, error) {
	uid := int64(0)
	defaultScrubPod := volume.NewPersistentVolumeRecyclerPodTemplate()
	// TODO: Move the recycler pods to dedicated namespace instead of polluting
	// openshift-infra.
	defaultScrubPod.Namespace = "openshift-infra"
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
		return nil, fmt.Errorf("could not create hostpath recycler pod from file %s: %+v", config.PersistentVolumeRecyclerConfiguration.PodTemplateFilePathHostPath, err)
	}
	allPlugins = append(allPlugins, host_path.ProbeVolumePlugins(hostPathConfig)...)

	nfsConfig := volume.VolumeConfig{
		RecyclerMinimumTimeout:   int(config.PersistentVolumeRecyclerConfiguration.MinimumTimeoutNFS),
		RecyclerTimeoutIncrement: int(config.PersistentVolumeRecyclerConfiguration.IncrementTimeoutNFS),
		RecyclerPodTemplate:      defaultScrubPod,
	}
	if err := kctrlmgr.AttemptToLoadRecycler(config.PersistentVolumeRecyclerConfiguration.PodTemplateFilePathNFS, &nfsConfig); err != nil {
		return nil, fmt.Errorf("could not create NFS recycler pod from file %s: %+v", config.PersistentVolumeRecyclerConfiguration.PodTemplateFilePathNFS, err)
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

	return allPlugins, nil
}
