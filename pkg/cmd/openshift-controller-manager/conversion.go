package openshift_controller_manager

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kcmoptions "k8s.io/kubernetes/cmd/kube-controller-manager/app/options"

	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	"github.com/openshift/origin/pkg/cmd/server/cm"
	cmdflags "github.com/openshift/origin/pkg/cmd/util/flags"
)

func ConvertMasterConfigToOpenshiftControllerConfig(input *configapi.MasterConfig) *configapi.OpenshiftControllerConfig {
	// this is the old flag binding logic
	flagOptions := kcmoptions.NewKubeControllerManagerOptions()
	flagOptions.Generic.ComponentConfig.LeaderElection.RetryPeriod = metav1.Duration{Duration: 3 * time.Second}
	flagFunc := cm.OriginControllerManagerAddFlags(flagOptions)
	errors := cmdflags.Resolve(input.KubernetesMasterConfig.ControllerArguments, flagFunc)
	if len(errors) > 0 {
		// this can't happen since we only run this on configs we have validated
		panic(errors)
	}

	// deep copy to make sure no linger references are shared
	in := input.DeepCopy()

	registryURLs := []string{}
	if len(in.ImagePolicyConfig.ExternalRegistryHostname) > 0 {
		registryURLs = append(registryURLs, in.ImagePolicyConfig.ExternalRegistryHostname)
	}
	if len(in.ImagePolicyConfig.InternalRegistryHostname) > 0 {
		registryURLs = append(registryURLs, in.ImagePolicyConfig.InternalRegistryHostname)
	}

	ret := &configapi.OpenshiftControllerConfig{
		ClientConnectionOverrides: in.MasterClients.OpenShiftLoopbackClientConnectionOverrides,
		ServingInfo:               &in.ServingInfo,
		Controllers:               in.ControllerConfig.Controllers,
		LeaderElection: configapi.LeaderElectionConfig{
			RetryPeriod:   flagOptions.Generic.ComponentConfig.LeaderElection.RetryPeriod,
			RenewDeadline: flagOptions.Generic.ComponentConfig.LeaderElection.RenewDeadline,
			LeaseDuration: flagOptions.Generic.ComponentConfig.LeaderElection.LeaseDuration,
		},
		HPA: configapi.HPAControllerConfig{
			DownscaleForbiddenWindow: flagOptions.Generic.ComponentConfig.HorizontalPodAutoscalerDownscaleForbiddenWindow,
			SyncPeriod:               flagOptions.Generic.ComponentConfig.HorizontalPodAutoscalerSyncPeriod,
			UpscaleForbiddenWindow:   flagOptions.Generic.ComponentConfig.HorizontalPodAutoscalerUpscaleForbiddenWindow,
		},
		ResourceQuota: configapi.ResourceQuotaControllerConfig{
			ConcurrentSyncs: flagOptions.Generic.ComponentConfig.ConcurrentResourceQuotaSyncs,
			SyncPeriod:      flagOptions.Generic.ComponentConfig.ResourceQuotaSyncPeriod,
			MinResyncPeriod: flagOptions.Generic.ComponentConfig.MinResyncPeriod,
		},
		ServiceServingCert: in.ControllerConfig.ServiceServingCert,
		Deployer: configapi.DeployerControllerConfig{
			ImageTemplateFormat: in.ImageConfig,
		},
		Build: configapi.BuildControllerConfig{
			ImageTemplateFormat: in.ImageConfig,
			// TODO bring in what need in a typed way.
			AdmissionPluginConfig: in.AdmissionConfig.PluginConfig,
		},
		ServiceAccount: configapi.ServiceAccountControllerConfig{
			ManagedNames: in.ServiceAccountConfig.ManagedNames,
		},
		DockerPullSecret: configapi.DockerPullSecretControllerConfig{
			RegistryURLs: registryURLs,
		},
		Network: configapi.NetworkControllerConfig{
			ClusterNetworks:    in.NetworkConfig.ClusterNetworks,
			NetworkPluginName:  in.NetworkConfig.NetworkPluginName,
			ServiceNetworkCIDR: in.NetworkConfig.ServiceNetworkCIDR,
		},
		Ingress: configapi.IngressControllerConfig{
			IngressIPNetworkCIDR: in.NetworkConfig.IngressIPNetworkCIDR,
		},
		SecurityAllocator: *in.ProjectConfig.SecurityAllocator,
		ImageImport: configapi.ImageImportControllerConfig{
			DisableScheduledImport:                     in.ImagePolicyConfig.DisableScheduledImport,
			MaxScheduledImageImportsPerMinute:          in.ImagePolicyConfig.MaxScheduledImageImportsPerMinute,
			ScheduledImageImportMinimumIntervalSeconds: in.ImagePolicyConfig.ScheduledImageImportMinimumIntervalSeconds,
		},
	}

	return ret
}
