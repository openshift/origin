package latest

import (
	"io/ioutil"
	"path"

	"github.com/ghodss/yaml"

	configapi "github.com/openshift/origin/pkg/cmd/server/api"
)

func ReadMasterConfig(filename string) (*configapi.MasterConfig, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	config := &configapi.MasterConfig{}

	if err := Codec.DecodeInto(data, config); err != nil {
		return nil, err
	}
	return config, nil
}

func ReadAndResolveMasterConfig(filename string) (*configapi.MasterConfig, error) {
	masterConfig, err := ReadMasterConfig(filename)
	if err != nil {
		return nil, err
	}

	configapi.ResolveMasterConfigPaths(masterConfig, path.Dir(filename))
	return masterConfig, nil
}

// WriteNode serializes the config to yaml.
func WriteNode(config *configapi.NodeConfig) ([]byte, error) {
	json, err := Codec.Encode(config)
	if err != nil {
		return nil, err
	}
	content, err := yaml.JSONToYAML(json)
	if err != nil {
		return nil, err
	}
	return content, nil
}
