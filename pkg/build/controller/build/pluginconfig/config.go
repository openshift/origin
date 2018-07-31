package pluginconfig

import (
	"io/ioutil"

	"github.com/ghodss/yaml"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"

	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
)

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

// GetPluginConfigObj gives the decoded object
func GetPluginConfigObj(pluginConfig map[string]*configapi.AdmissionPluginConfig, pluginName string, scheme *runtime.Scheme) (runtime.Object, error) {
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
