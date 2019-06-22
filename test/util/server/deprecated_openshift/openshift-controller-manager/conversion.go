package openshift_controller_manager

import (
	"fmt"
	"io/ioutil"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"
	apiserverflag "k8s.io/component-base/cli/flag"
	kcmapp "k8s.io/kubernetes/cmd/kube-controller-manager/app"
	kcmoptions "k8s.io/kubernetes/cmd/kube-controller-manager/app/options"

	configv1 "github.com/openshift/api/config/v1"
	legacyconfigv1 "github.com/openshift/api/legacyconfig/v1"
	openshiftcontrolplanev1 "github.com/openshift/api/openshiftcontrolplane/v1"
	cmdflags "github.com/openshift/openshift-apiserver/pkg/cmd/openshift-apiserver/openshiftapiserver/configprocessing/flags"
	"github.com/openshift/origin/test/util/server/deprecated_openshift/configconversion"
)

func ConvertMasterConfigToOpenshiftControllerConfig(input *legacyconfigv1.MasterConfig) *openshiftcontrolplanev1.OpenShiftControllerManagerConfig {
	// this is the old flag binding logic
	flagOptions, err := kcmoptions.NewKubeControllerManagerOptions()
	if err != nil {
		// coder error
		panic(err)
	}
	flagOptions.Generic.LeaderElection.RetryPeriod = metav1.Duration{Duration: 3 * time.Second}
	flagFunc := originControllerManagerAddFlags(flagOptions)
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

	servingInfo, err := configconversion.ToHTTPServingInfo(&in.ServingInfo)
	if err != nil {
		// this should happen on scrubbed input
		panic(err)
	}

	ret := &openshiftcontrolplanev1.OpenShiftControllerManagerConfig{
		KubeClientConfig: configv1.KubeClientConfig{
			KubeConfig: in.MasterClients.OpenShiftLoopbackKubeConfig,
		},
		ServingInfo: &servingInfo,
		Controllers: in.ControllerConfig.Controllers,
		LeaderElection: configv1.LeaderElection{
			Namespace:     "kube-system",
			Name:          "openshift-master-controllers",
			RetryPeriod:   flagOptions.Generic.LeaderElection.RetryPeriod,
			RenewDeadline: flagOptions.Generic.LeaderElection.RenewDeadline,
			LeaseDuration: flagOptions.Generic.LeaderElection.LeaseDuration,
		},
		ResourceQuota: openshiftcontrolplanev1.ResourceQuotaControllerConfig{
			ConcurrentSyncs: flagOptions.ResourceQuotaController.ConcurrentResourceQuotaSyncs,
			SyncPeriod:      flagOptions.ResourceQuotaController.ResourceQuotaSyncPeriod,
			MinResyncPeriod: flagOptions.Generic.MinResyncPeriod,
		},
		ServiceServingCert: openshiftcontrolplanev1.ServiceServingCert{
			Signer: &configv1.CertInfo{
				CertFile: in.ControllerConfig.ServiceServingCert.Signer.CertFile,
				KeyFile:  in.ControllerConfig.ServiceServingCert.Signer.KeyFile,
			},
		},
		Deployer: openshiftcontrolplanev1.DeployerControllerConfig{
			ImageTemplateFormat: openshiftcontrolplanev1.ImageConfig{
				Format: in.ImageConfig.Format,
				Latest: in.ImageConfig.Latest,
			},
		},
		Build: openshiftcontrolplanev1.BuildControllerConfig{
			ImageTemplateFormat: openshiftcontrolplanev1.ImageConfig{
				Format: in.ImageConfig.Format,
				Latest: in.ImageConfig.Latest,
			},

			BuildDefaults:  buildDefaults,
			BuildOverrides: buildOverrides,
		},
		ServiceAccount: openshiftcontrolplanev1.ServiceAccountControllerConfig{
			ManagedNames: in.ServiceAccountConfig.ManagedNames,
		},
		DockerPullSecret: openshiftcontrolplanev1.DockerPullSecretControllerConfig{
			RegistryURLs: registryURLs,
		},
		Network: openshiftcontrolplanev1.NetworkControllerConfig{
			NetworkPluginName:  in.NetworkConfig.NetworkPluginName,
			ServiceNetworkCIDR: in.NetworkConfig.ServiceNetworkCIDR,
			VXLANPort:          in.NetworkConfig.VXLANPort,
		},
		Ingress: openshiftcontrolplanev1.IngressControllerConfig{
			IngressIPNetworkCIDR: in.NetworkConfig.IngressIPNetworkCIDR,
		},
		SecurityAllocator: openshiftcontrolplanev1.SecurityAllocator{
			UIDAllocatorRange:   in.ProjectConfig.SecurityAllocator.UIDAllocatorRange,
			MCSAllocatorRange:   in.ProjectConfig.SecurityAllocator.MCSAllocatorRange,
			MCSLabelsPerProject: in.ProjectConfig.SecurityAllocator.MCSLabelsPerProject,
		},
		ImageImport: openshiftcontrolplanev1.ImageImportControllerConfig{
			DisableScheduledImport:                     in.ImagePolicyConfig.DisableScheduledImport,
			MaxScheduledImageImportsPerMinute:          in.ImagePolicyConfig.MaxScheduledImageImportsPerMinute,
			ScheduledImageImportMinimumIntervalSeconds: in.ImagePolicyConfig.ScheduledImageImportMinimumIntervalSeconds,
		},
	}

	if in.MasterClients.OpenShiftLoopbackClientConnectionOverrides != nil {
		ret.KubeClientConfig.ConnectionOverrides.AcceptContentTypes = in.MasterClients.OpenShiftLoopbackClientConnectionOverrides.AcceptContentTypes
		ret.KubeClientConfig.ConnectionOverrides.Burst = in.MasterClients.OpenShiftLoopbackClientConnectionOverrides.Burst
		ret.KubeClientConfig.ConnectionOverrides.ContentType = in.MasterClients.OpenShiftLoopbackClientConnectionOverrides.ContentType
		ret.KubeClientConfig.ConnectionOverrides.QPS = in.MasterClients.OpenShiftLoopbackClientConnectionOverrides.QPS
	}

	for _, curr := range in.NetworkConfig.ClusterNetworks {
		ret.Network.ClusterNetworks = append(ret.Network.ClusterNetworks, openshiftcontrolplanev1.ClusterNetworkEntry{
			CIDR:             curr.CIDR,
			HostSubnetLength: curr.HostSubnetLength,
		})
	}

	return ret
}

// getBuildDefaults creates a new BuildDefaults that will apply the defaults specified in the plugin config
func getBuildDefaults(pluginConfig map[string]*legacyconfigv1.AdmissionPluginConfig) (*openshiftcontrolplanev1.BuildDefaultsConfig, error) {
	const buildDefaultsPlugin = "BuildDefaults"
	uncastConfig, err := getPluginConfigObj(pluginConfig, buildDefaultsPlugin, &openshiftcontrolplanev1.BuildDefaultsConfig{})
	if err != nil {
		return nil, err
	}
	if uncastConfig == nil {
		return nil, nil
	}
	config, ok := uncastConfig.(*openshiftcontrolplanev1.BuildDefaultsConfig)
	if !ok {
		return nil, fmt.Errorf("expected BuildDefaultsConfig, not %T", uncastConfig)
	}

	return config, nil
}

// getBuildOverrides creates a new BuildOverrides that will apply the overrides specified in the plugin config
func getBuildOverrides(pluginConfig map[string]*legacyconfigv1.AdmissionPluginConfig) (*openshiftcontrolplanev1.BuildOverridesConfig, error) {
	const buildOverridesPlugin = "BuildOverrides"
	uncastConfig, err := getPluginConfigObj(pluginConfig, buildOverridesPlugin, &openshiftcontrolplanev1.BuildOverridesConfig{})
	if err != nil {
		return nil, err
	}
	if uncastConfig == nil {
		return nil, err
	}
	config, ok := uncastConfig.(*openshiftcontrolplanev1.BuildOverridesConfig)
	if !ok {
		return nil, fmt.Errorf("expected BuildDefaultsConfig, not %T", uncastConfig)
	}

	return config, nil
}

func getPluginConfigObj(pluginConfig map[string]*legacyconfigv1.AdmissionPluginConfig, pluginName string, target runtime.Object) (runtime.Object, error) {
	yamlContent, err := getPluginConfigYAML(pluginConfig, pluginName)
	if err != nil {
		return nil, err
	}
	if len(yamlContent) == 0 {
		return nil, nil
	}

	jsonData, err := kyaml.ToJSON(yamlContent)
	if err != nil {
		return nil, err
	}
	uncastObj, err := runtime.Decode(unstructured.UnstructuredJSONScheme, jsonData)
	if err != nil {
		return nil, err
	}
	uncastObj.(*unstructured.Unstructured).Object["apiVersion"] = openshiftcontrolplanev1.GroupName + "/v1"
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(uncastObj.(runtime.Unstructured).UnstructuredContent(), target); err != nil {
		return nil, err
	}
	return target, nil
}

// getPluginConfigYAML gives the byte content of the config for a given plugin
func getPluginConfigYAML(pluginConfig map[string]*legacyconfigv1.AdmissionPluginConfig, pluginName string) ([]byte, error) {
	// Check whether a config is specified for this plugin. If not, default to the
	// global plugin config file (if any).
	cfg, hasConfig := pluginConfig[pluginName]
	if !hasConfig {
		return nil, nil
	}
	switch {
	case len(cfg.Configuration.Raw) == 0 && len(cfg.Location) == 0:
		return nil, fmt.Errorf("missing both config and location")
	case len(cfg.Configuration.Raw) == 0:
		return ioutil.ReadFile(cfg.Location)
	default:
		return cfg.Configuration.Raw, nil
	}
}

func originControllerManagerAddFlags(cmserver *kcmoptions.KubeControllerManagerOptions) apiserverflag.NamedFlagSets {
	return cmserver.Flags(kcmapp.KnownControllers(), kcmapp.ControllersDisabledByDefault.List())
}
