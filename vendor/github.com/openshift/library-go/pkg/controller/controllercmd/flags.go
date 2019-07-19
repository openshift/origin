package controllercmd

import (
	"encoding/json"
	"fmt"
	"io/ioutil"

	"github.com/spf13/cobra"

	"github.com/openshift/library-go/pkg/config/client"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/rest"
)

// ControllerFlags provides the "normal" controller flags
type ControllerFlags struct {
	// ConfigFile hold the configfile to load
	ConfigFile string
	// KubeConfigFile points to a kubeconfig file if you don't want to use the in cluster config
	KubeConfigFile string
}

// NewControllerFlags returns flags with default values set
func NewControllerFlags() *ControllerFlags {
	return &ControllerFlags{}
}

// Validate makes sure the required flags are specified and no illegal combinations are found
func (o *ControllerFlags) Validate() error {
	// everything is optional currently
	return nil
}

// AddFlags register and binds the default flags
func (f *ControllerFlags) AddFlags(cmd *cobra.Command) {
	flags := cmd.Flags()
	// This command only supports reading from config
	flags.StringVar(&f.ConfigFile, "config", f.ConfigFile, "Location of the master configuration file to run from.")
	cmd.MarkFlagFilename("config", "yaml", "yml")
	flags.StringVar(&f.KubeConfigFile, "kubeconfig", f.KubeConfigFile, "Location of the master configuration file to run from.")
	cmd.MarkFlagFilename("kubeconfig", "kubeconfig")
}

// ToConfigObj given completed flags, returns a config object for the flag that was specified.
// TODO versions goes away in 1.11
func (f *ControllerFlags) ToConfigObj() ([]byte, *unstructured.Unstructured, error) {
	// no file means empty, not err
	if len(f.ConfigFile) == 0 {
		return nil, nil, nil
	}

	content, err := ioutil.ReadFile(f.ConfigFile)
	if err != nil {
		return nil, nil, err
	}
	// empty file means empty, not err
	if len(content) == 0 {
		return nil, nil, err
	}

	data, err := kyaml.ToJSON(content)
	if err != nil {
		return nil, nil, err
	}
	uncastObj, err := runtime.Decode(unstructured.UnstructuredJSONScheme, data)
	if err != nil {
		return nil, nil, err
	}

	return content, uncastObj.(*unstructured.Unstructured), nil
}

// ToClientConfig given completed flags, returns a rest.Config.  overrides are optional
func (f *ControllerFlags) ToClientConfig(overrides *client.ClientConnectionOverrides) (*rest.Config, error) {
	return client.GetKubeConfigOrInClusterConfig(f.KubeConfigFile, overrides)
}

// ReadYAML decodes a runtime.Object from the provided scheme
// TODO versions goes away with more complete scheme in 1.11
func ReadYAML(data []byte, configScheme *runtime.Scheme, versions ...schema.GroupVersion) (runtime.Object, error) {
	data, err := kyaml.ToJSON(data)
	if err != nil {
		return nil, err
	}
	configCodecFactory := serializer.NewCodecFactory(configScheme)
	obj, err := runtime.Decode(configCodecFactory.UniversalDecoder(versions...), data)
	if err != nil {
		return nil, captureSurroundingJSONForError("error reading config: ", data, err)
	}
	return obj, err
}

// ReadYAMLFile read a file and decodes a runtime.Object from the provided scheme
func ReadYAMLFile(filename string, configScheme *runtime.Scheme, versions ...schema.GroupVersion) (runtime.Object, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	obj, err := ReadYAML(data, configScheme, versions...)
	if err != nil {
		return nil, fmt.Errorf("could not load config file %q due to an error: %v", filename, err)
	}
	return obj, err
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
