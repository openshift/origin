package latest

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"path"
	"reflect"

	"github.com/ghodss/yaml"

	"k8s.io/kubernetes/pkg/runtime"
	kyaml "k8s.io/kubernetes/pkg/util/yaml"

	configapi "github.com/openshift/origin/pkg/cmd/server/api"
)

func ReadSessionSecrets(filename string) (*configapi.SessionSecrets, error) {
	config := &configapi.SessionSecrets{}
	if err := ReadYAMLFileInto(filename, config); err != nil {
		return nil, err
	}
	return config, nil
}

func ReadMasterConfig(filename string) (*configapi.MasterConfig, error) {
	config := &configapi.MasterConfig{}
	if err := ReadYAMLFileInto(filename, config); err != nil {
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
	if err := ReadYAMLFileInto(filename, config); err != nil {
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

// TODO: Remove this when a YAML serializer is available from upstream
func WriteYAML(obj runtime.Object) ([]byte, error) {
	json, err := runtime.Encode(Codec, obj)
	if err != nil {
		return nil, err
	}

	content, err := yaml.JSONToYAML(json)
	if err != nil {
		return nil, err
	}
	return content, err
}

func ReadYAML(reader io.Reader) (runtime.Object, error) {
	if reader == nil || reflect.ValueOf(reader).IsNil() {
		return nil, nil
	}
	data, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	jsonData, err := kyaml.ToJSON(data)
	if err != nil {
		return nil, err
	}
	return runtime.Decode(Codec, jsonData)
}

// TODO: Remove this when a YAML serializer is available from upstream
func ReadYAMLInto(data []byte, obj runtime.Object) error {
	data, err := kyaml.ToJSON(data)
	if err != nil {
		return err
	}
	err = runtime.DecodeInto(Codec, data, obj)
	return captureSurroundingJSONForError("error reading config: ", data, err)
}

func ReadYAMLFileInto(filename string, obj runtime.Object) error {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}
	err = ReadYAMLInto(data, obj)
	if err != nil {
		return fmt.Errorf("could not load config file %q due to an error: %v", filename, err)
	}
	return nil
}

// TODO: we ultimately want a better decoder for JSON that allows us exact line numbers and better
// surrounding text description. This should be removed / replaced when that happens.
func captureSurroundingJSONForError(prefix string, data []byte, err error) error {
	if syntaxErr, ok := err.(*json.SyntaxError); err != nil && ok {
		offset := syntaxErr.Offset
		begin := offset - 20
		if begin < 0 {
			begin = 0
		}
		end := offset + 20
		if end > int64(len(data)) {
			end = int64(len(data))
		}
		return fmt.Errorf("%s%v (found near '%s')", prefix, err, string(data[begin:end]))
	}
	if err != nil {
		return fmt.Errorf("%s%v", prefix, err)
	}
	return err
}

// IsAdmissionPluginActivated returns true if the admission plugin is activated using configapi.DefaultAdmissionConfig
// otherwise it returns a default value
func IsAdmissionPluginActivated(reader io.Reader, defaultValue bool) (bool, error) {
	obj, err := ReadYAML(reader)
	if err != nil {
		return false, err
	}
	if obj == nil {
		return defaultValue, nil
	}
	activationConfig, ok := obj.(*configapi.DefaultAdmissionConfig)
	if !ok {
		// if we failed the cast, then we've got a config object specified for this admission plugin
		// that means that this must be enabled and all additional validation is up to the
		// admission plugin itself
		return true, nil
	}

	return !activationConfig.Disable, nil
}
