package openshift_controller_manager

import (
	"fmt"
	"io/ioutil"
	"time"

	"github.com/ghodss/yaml"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"
	kcmoptions "k8s.io/kubernetes/cmd/kube-controller-manager/app/options"

	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	configv1 "github.com/openshift/origin/pkg/cmd/server/apis/config/v1"
	"github.com/openshift/origin/pkg/cmd/server/cm"
	cmdflags "github.com/openshift/origin/pkg/cmd/util/flags"
)

func ConvertMasterConfigToOpenshiftControllerConfig(input *configapi.MasterConfig) *configapi.OpenshiftControllerConfig {
	// this is the old flag binding logic
	flagOptions, err := kcmoptions.NewKubeControllerManagerOptions()
	if err != nil {
		// coder error
		panic(err)
	}
	flagOptions.GenericComponent.LeaderElection.RetryPeriod = metav1.Duration{Duration: 3 * time.Second}
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

	buildDefaults, err := getBuildDefaults(in.AdmissionConfig.PluginConfig)
	if err != nil {
		// this should happen on scrubbed input
		panic(err)
	}
	buildOverrides, err := getBuildOverrides(in.AdmissionConfig.PluginConfig)
	if err != nil {
		// this should happen on scrubbed input
		panic(err)
	}

	ret := &configapi.OpenshiftControllerConfig{
		ClientConnectionOverrides: in.MasterClients.OpenShiftLoopbackClientConnectionOverrides,
		ServingInfo:               &in.ServingInfo,
		Controllers:               in.ControllerConfig.Controllers,
		LeaderElection: configapi.LeaderElectionConfig{
			RetryPeriod:   flagOptions.GenericComponent.LeaderElection.RetryPeriod,
			RenewDeadline: flagOptions.GenericComponent.LeaderElection.RenewDeadline,
			LeaseDuration: flagOptions.GenericComponent.LeaderElection.LeaseDuration,
		},
		ResourceQuota: configapi.ResourceQuotaControllerConfig{
			ConcurrentSyncs: flagOptions.ResourceQuotaController.ConcurrentResourceQuotaSyncs,
			SyncPeriod:      flagOptions.ResourceQuotaController.ResourceQuotaSyncPeriod,
			MinResyncPeriod: flagOptions.GenericComponent.MinResyncPeriod,
		},
		ServiceServingCert: in.ControllerConfig.ServiceServingCert,
		Deployer: configapi.DeployerControllerConfig{
			ImageTemplateFormat: in.ImageConfig,
		},
		Build: configapi.BuildControllerConfig{
			ImageTemplateFormat: in.ImageConfig,

			BuildDefaults:  buildDefaults,
			BuildOverrides: buildOverrides,
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
			VXLANPort:          in.NetworkConfig.VXLANPort,
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

// getBuildDefaults creates a new BuildDefaults that will apply the defaults specified in the plugin config
func getBuildDefaults(pluginConfig map[string]*configapi.AdmissionPluginConfig) (*configapi.BuildDefaultsConfig, error) {
	const buildDefaultsPlugin = "BuildDefaults"
	scheme := runtime.NewScheme()
	configv1.InstallLegacy(scheme)
	uncastConfig, err := getPluginConfigObj(pluginConfig, buildDefaultsPlugin, scheme)
	if err != nil {
		return nil, err
	}
	if uncastConfig == nil {
		return nil, nil
	}
	config, ok := uncastConfig.(*configapi.BuildDefaultsConfig)
	if !ok {
		return nil, fmt.Errorf("expected BuildDefaultsConfig, not %T", uncastConfig)
	}

	return config, nil
}

// getBuildOverrides creates a new BuildOverrides that will apply the overrides specified in the plugin config
func getBuildOverrides(pluginConfig map[string]*configapi.AdmissionPluginConfig) (*configapi.BuildOverridesConfig, error) {
	const buildOverridesPlugin = "BuildOverrides"
	scheme := runtime.NewScheme()
	configv1.InstallLegacy(scheme)
	uncastConfig, err := getPluginConfigObj(pluginConfig, buildOverridesPlugin, scheme)
	if err != nil {
		return nil, err
	}
	if uncastConfig == nil {
		return nil, err
	}
	config, ok := uncastConfig.(*configapi.BuildOverridesConfig)
	if !ok {
		return nil, fmt.Errorf("expected BuildDefaultsConfig, not %T", uncastConfig)
	}

	return config, nil
}

func getPluginConfigObj(pluginConfig map[string]*configapi.AdmissionPluginConfig, pluginName string, scheme *runtime.Scheme) (runtime.Object, error) {
	yamlContent, err := getPluginConfigYAML(pluginConfig, pluginName, scheme)
	if err != nil {
		return nil, err
	}
	if len(yamlContent) == 0 {
		return nil, nil
	}

	internalDecoder := serializer.NewCodecFactory(scheme).UniversalDecoder()
	jsonData, err := kyaml.ToJSON(yamlContent)
	if err != nil {
		return nil, err
	}
	return runtime.Decode(internalDecoder, jsonData)
}

// getPluginConfigYAML gives the byte content of the config for a given plugin
func getPluginConfigYAML(pluginConfig map[string]*configapi.AdmissionPluginConfig, pluginName string, scheme *runtime.Scheme) ([]byte, error) {
	// Check whether a config is specified for this plugin. If not, default to the
	// global plugin config file (if any).
	cfg, hasConfig := pluginConfig[pluginName]
	if !hasConfig {
		return nil, nil
	}
	obj := cfg.Configuration
	if obj == nil {
		return ioutil.ReadFile(cfg.Location)
	}

	codec := serializer.NewCodecFactory(scheme).LegacyCodec(scheme.PrioritizedVersionsAllGroups()...)
	json, err := runtime.Encode(codec, obj)
	if err != nil {
		return nil, err
	}

	return yaml.JSONToYAML(json)
}
