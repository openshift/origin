package pluginconfig

import (
	"io/ioutil"
	"os"

	"k8s.io/apimachinery/pkg/apimachinery/announced"
	"k8s.io/apimachinery/pkg/apimachinery/registered"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"
	kapiserverinstall "k8s.io/apiserver/pkg/apis/apiserver/install"
	kapiserverv1alpha1 "k8s.io/apiserver/pkg/apis/apiserver/v1alpha1"

	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/api/latest"
)

var (
	groupFactoryRegistry = make(announced.APIGroupFactoryRegistry)
	registry             = registered.NewOrDie("")
	scheme               = runtime.NewScheme()
	codecs               = serializer.NewCodecFactory(scheme)
)

func init() {
	kapiserverinstall.Install(groupFactoryRegistry, registry, scheme)
}

func GetPluginConfig(cfg configapi.AdmissionPluginConfig) (string, error) {
	obj := cfg.Configuration
	if obj == nil {
		return cfg.Location, nil
	}

	configFile, err := ioutil.TempFile("", "admission-plugin-config")
	if err != nil {
		return "", err
	}
	if err = configFile.Close(); err != nil {
		return "", err
	}
	content, err := latest.WriteYAML(obj)
	if err != nil {
		return "", err
	}
	if err = ioutil.WriteFile(configFile.Name(), content, 0644); err != nil {
		return "", err
	}
	return configFile.Name(), nil
}

// GetPluginConfigFile translates from the master plugin config to a file name containing
// a particular plugin's config (the file may be a temp file if config is embedded)
func GetPluginConfigFile(pluginConfig map[string]configapi.AdmissionPluginConfig, pluginName string, defaultConfigFilePath string) (string, error) {
	// Check whether a config is specified for this plugin. If not, default to the
	// global plugin config file (if any).
	if cfg, hasConfig := pluginConfig[pluginName]; hasConfig {
		configFilePath, err := GetPluginConfig(cfg)
		if err != nil {
			return "", err
		}
		return configFilePath, nil
	}
	return defaultConfigFilePath, nil
}

func GetAdmissionConfigurationConfig(pluginName string, cfg configapi.AdmissionPluginConfig) (string, error) {
	if cfg.Configuration == nil && cfg.Location == "" {
		return "", nil
	}

	var cfgJSON []byte
	if cfg.Configuration == nil {
		f, err := os.Open(cfg.Location)
		if err != nil {
			return "", err
		}
		data, err := ioutil.ReadAll(f)
		if err != nil {
			return "", err
		}
		cfgJSON, err = kyaml.ToJSON(data)
		if err != nil {
			return "", err
		}
	} else {
		var err error
		cfgJSON, err = runtime.Encode(latest.Codec, cfg.Configuration)
		if err != nil {
			return "", err
		}
	}

	// convert into versioned admission plugin configuration
	obj := kapiserverv1alpha1.AdmissionConfiguration{
		Plugins: []kapiserverv1alpha1.AdmissionPluginConfiguration{
			{
				Name:          pluginName,
				Configuration: runtime.RawExtension{Raw: cfgJSON},
			},
		},
	}

	configFile, err := ioutil.TempFile("", "admission-plugin-config")
	if err != nil {
		return "", err
	}
	if err = configFile.Close(); err != nil {
		return "", err
	}
	codec := codecs.LegacyCodec(kapiserverv1alpha1.SchemeGroupVersion)
	data, err := runtime.Encode(codec, &obj)
	if err != nil {
		return "", err
	}
	if err = ioutil.WriteFile(configFile.Name(), data, 0644); err != nil {
		return "", err
	}
	return configFile.Name(), nil
}

// GetAdmissionConfigurationFile translates from the master plugin config to a file name containing
// a particular plugin's config (the file may be a temp file if config is embedded), embedded in
// k8s.io/apiserver/pkg/apis/apiserver.AdmissionConfiguration.
func GetAdmissionConfigurationFile(pluginConfig map[string]configapi.AdmissionPluginConfig, pluginName string, defaultConfigFilePath string) (string, error) {
	// Check whether a config is specified for this plugin. If not, default to the
	// global plugin config file (if any).
	if cfg, hasConfig := pluginConfig[pluginName]; hasConfig {
		configFilePath, err := GetAdmissionConfigurationConfig(pluginName, cfg)
		if err != nil {
			return "", err
		}
		return configFilePath, nil
	}
	return defaultConfigFilePath, nil
}
