package openshiftadmission

import (
	"io/ioutil"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/apis/apiserver"

	configv1 "github.com/openshift/api/config/v1"
	oadmission "github.com/openshift/origin/pkg/cmd/server/admission"
	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	configapilatest "github.com/openshift/origin/pkg/cmd/server/apis/config/latest"
)

func NewAdmissionChains(
	explicitOn, explicitOff []string,
	pluginConfig map[string]configv1.AdmissionPluginConfig,
	admissionInitializer admission.PluginInitializer,
	admissionDecorator admission.Decorator,
) (admission.Interface, error) {
	upstreamAdmissionConfig, err := ConvertOpenshiftAdmissionConfigToKubeAdmissionConfig(pluginConfig)
	if err != nil {
		return nil, err
	}
	configBytes, err := configapilatest.WriteYAML(upstreamAdmissionConfig)
	if err != nil {
		return nil, err
	}

	tempFile, err := ioutil.TempFile("", "master-config.yaml")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tempFile.Name())
	if _, err := tempFile.Write(configBytes); err != nil {
		return nil, err
	}
	tempFile.Close()
	admissionPluginConfigFilename := tempFile.Name()

	allOffPlugins := append(DefaultOffPlugins.List(), explicitOff...)
	disabledPlugins := sets.NewString(allOffPlugins...)
	enabledPlugins := sets.NewString(explicitOn...)
	disabledPlugins = disabledPlugins.Difference(enabledPlugins)
	orderedPlugins := []string{}
	for _, plugin := range OpenShiftAdmissionPlugins {
		if !disabledPlugins.Has(plugin) {
			orderedPlugins = append(orderedPlugins, plugin)
		}
	}
	admissionChain, err := newAdmissionChainFunc(orderedPlugins, admissionPluginConfigFilename, admissionInitializer, admissionDecorator)
	if err != nil {
		return nil, err
	}

	return admissionChain, err
}

// newAdmissionChainFunc is for unit testing only.  You should NEVER OVERRIDE THIS outside of a unit test.
var newAdmissionChainFunc = newAdmissionChain

func newAdmissionChain(pluginNames []string, admissionConfigFilename string, admissionInitializer admission.PluginInitializer, admissionDecorator admission.Decorator) (admission.Interface, error) {
	plugins := []admission.Interface{}
	for _, pluginName := range pluginNames {
		var (
			plugin admission.Interface
		)

		// TODO this needs to be refactored to use the admission scheme we created upstream.  I think this holds us for the rebase.
		pluginsConfigProvider, err := admission.ReadAdmissionConfiguration([]string{pluginName}, admissionConfigFilename, configapi.Scheme)
		if err != nil {
			return nil, err
		}
		plugin, err = OriginAdmissionPlugins.NewFromPlugins([]string{pluginName}, pluginsConfigProvider, admissionInitializer, admissionDecorator)
		if err != nil {
			// should have been caught with validation
			return nil, err
		}
		if plugin == nil {
			continue
		}

		plugins = append(plugins, plugin)

	}

	// ensure that plugins have been properly initialized
	if err := oadmission.Validate(plugins); err != nil {
		return nil, err
	}

	return admission.NewChainHandler(plugins...), nil
}

func ConvertOpenshiftAdmissionConfigToKubeAdmissionConfig(in map[string]configv1.AdmissionPluginConfig) (*apiserver.AdmissionConfiguration, error) {
	ret := &apiserver.AdmissionConfiguration{}

	for _, pluginName := range sets.StringKeySet(in).List() {
		openshiftConfig := in[pluginName]

		kubeConfig := apiserver.AdmissionPluginConfiguration{
			Name: pluginName,
			Path: openshiftConfig.Location,
		}

		kubeConfig.Configuration = &runtime.Unknown{
			Raw: openshiftConfig.Configuration.Raw,
		}
		ret.Plugins = append(ret.Plugins, kubeConfig)
	}

	return ret, nil
}
