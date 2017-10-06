package pluginconfig

import (
	"io/ioutil"

	"k8s.io/apimachinery/pkg/apimachinery/announced"
	"k8s.io/apimachinery/pkg/apimachinery/registered"
	"k8s.io/apimachinery/pkg/runtime"
	kapiserverinstall "k8s.io/apiserver/pkg/apis/apiserver/install"

	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/api/latest"
)

var (
	groupFactoryRegistry = make(announced.APIGroupFactoryRegistry)
	registry             = registered.NewOrDie("")
	scheme               = runtime.NewScheme()
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
