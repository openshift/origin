package latest

import (
	"io/ioutil"
	"path"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	kyaml "github.com/GoogleCloudPlatform/kubernetes/pkg/util/yaml"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"

	"github.com/ghodss/yaml"
)

func ReadSessionSecrets(filename string) (*configapi.SessionSecrets, error) {
	config := &configapi.SessionSecrets{}
	if err := ReadYAMLFile(filename, config); err != nil {
		return nil, err
	}
	return config, nil
}

func ReadMasterConfig(filename string) (*configapi.MasterConfig, error) {
	config := &configapi.MasterConfig{}
	if err := ReadYAMLFile(filename, config); err != nil {
		return nil, err
	}
	return config, nil
}

func ReadAndResolveMasterConfig(filename string) (*configapi.MasterConfig, error) {
	masterConfig, err := ReadMasterConfig(filename)
	if err != nil {
		return nil, err
	}

	if err := configapi.ResolveMasterConfigPaths(masterConfig, path.Dir(filename)); err != nil {
		return nil, err
	}

	return masterConfig, nil
}

func ReadNodeConfig(filename string) (*configapi.NodeConfig, error) {
	config := &configapi.NodeConfig{}
	if err := ReadYAMLFile(filename, config); err != nil {
		return nil, err
	}
	return config, nil
}

func ReadAndResolveNodeConfig(filename string) (*configapi.NodeConfig, error) {
	nodeConfig, err := ReadNodeConfig(filename)
	if err != nil {
		return nil, err
	}

	if err := configapi.ResolveNodeConfigPaths(nodeConfig, path.Dir(filename)); err != nil {
		return nil, err
	}

	return nodeConfig, nil
}

func WriteYAML(obj runtime.Object) ([]byte, error) {
	json, err := Codec.Encode(obj)
	if err != nil {
		return nil, err
	}

	content, err := yaml.JSONToYAML(json)
	if err != nil {
		return nil, err
	}
	return content, err
}

func ReadYAMLFile(filename string, obj runtime.Object) error {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}
	data, err = kyaml.ToJSON(data, false)
	if err != nil {
		return err
	}
	return Codec.DecodeInto(data, obj)
}
