package latest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path"

	"k8s.io/klog"

	"k8s.io/apimachinery/pkg/runtime"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"

	legacyconfigv1 "github.com/openshift/api/legacyconfig/v1"
	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
)

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
func ReadYAMLInto(data []byte, obj runtime.Object) error {
	jsonData, err := kyaml.ToJSON(data)
	if err != nil {
		return err
	}
	if err := runtime.DecodeInto(Codec, jsonData, obj); err != nil {
		return captureSurroundingJSONForError("error reading config: ", jsonData, err)
	}
	// make sure there are no extra fields in jsonData
	return strictDecodeCheck(jsonData, obj)
}

// strictDecodeCheck fails decodes when jsonData contains fields not included in the external version of obj
func strictDecodeCheck(jsonData []byte, obj runtime.Object) error {
	out, err := getExternalZeroValue(obj) // we need the external version of obj as that has the correct JSON struct tags
	if err != nil {
		klog.Errorf("Encountered config error %v in object %T, raw JSON:\n%s", err, obj, string(jsonData)) // TODO just return the error and die
		// never error for now, we need to determine a safe way to make this check fatal
		return nil
	}
	d := json.NewDecoder(bytes.NewReader(jsonData))
	d.DisallowUnknownFields()
	// note that we only care about the error, out is discarded
	if err := d.Decode(out); err != nil {
		klog.Errorf("Encountered config error %v in object %T, raw JSON:\n%s", err, obj, string(jsonData)) // TODO just return the error and die
	}
	// never error for now, we need to determine a safe way to make this check fatal
	return nil
}

// getExternalZeroValue returns the zero value of the external version of obj
func getExternalZeroValue(obj runtime.Object) (runtime.Object, error) {
	gvks, _, err := configapi.Scheme.ObjectKinds(obj)
	if err != nil {
		return nil, err
	}
	if len(gvks) == 0 { // should never happen
		return nil, fmt.Errorf("no gvks found for %#v", obj)
	}
	gvk := legacyconfigv1.LegacySchemeGroupVersion.WithKind(gvks[0].Kind)
	return configapi.Scheme.New(gvk)
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
