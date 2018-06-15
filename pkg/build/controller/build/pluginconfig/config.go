package pluginconfig

import (
	"io/ioutil"

	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/apimachinery/announced"
	"k8s.io/apimachinery/pkg/apimachinery/registered"
	"k8s.io/apimachinery/pkg/runtime"
	kapiserverinstall "k8s.io/apiserver/pkg/apis/apiserver/install"

	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	"github.com/openshift/origin/pkg/cmd/server/apis/config/latest"
	configlatest "github.com/openshift/origin/pkg/cmd/server/apis/config/latest"
)

var (
	groupFactoryRegistry = make(announced.APIGroupFactoryRegistry)
	registry             = registered.NewOrDie("")
	scheme               = runtime.NewScheme()
)

func init() {
	kapiserverinstall.Install(groupFactoryRegistry, registry, scheme)
}

func getPluginConfig(cfg configapi.AdmissionPluginConfig) (string, error) {
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

// getPluginConfigFile translates from the master plugin config to a file name containing
// a particular plugin's config (the file may be a temp file if config is embedded)
func getPluginConfigFile(pluginConfig map[string]*configapi.AdmissionPluginConfig, pluginName string, defaultConfigFilePath string) (string, error) {
	// Check whether a config is specified for this plugin. If not, default to the
	// global plugin config file (if any).
	if cfg, hasConfig := pluginConfig[pluginName]; hasConfig {
		configFilePath, err := getPluginConfig(*cfg)
		if err != nil {
			return "", err
		}
		return configFilePath, nil
	}
	return defaultConfigFilePath, nil
}

// ReadPluginConfig will read a plugin configuration object from a reader stream
func ReadPluginConfig(pluginConfig map[string]*configapi.AdmissionPluginConfig, name string, config runtime.Object) error {
	configFilePath, err := getPluginConfigFile(pluginConfig, name, "")
	if err != nil || len(configFilePath) == 0 {
		return err
	}

	err = configlatest.ReadYAMLFileInto(configFilePath, config)
	if err != nil {
		glog.Errorf("couldn't open plugin configuration %s: %#v", configFilePath, err)
		return err
	}
	return nil
}
