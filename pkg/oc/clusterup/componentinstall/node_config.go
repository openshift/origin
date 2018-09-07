package componentinstall

import (
	"fmt"
	"io/ioutil"

	"github.com/ghodss/yaml"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	configv1 "github.com/openshift/origin/pkg/cmd/server/apis/config/v1"
)

var (
	nodeConfigScheme  = runtime.NewScheme()
	nodeConfigDecoder runtime.Decoder
	nodeConfigEncoder runtime.Encoder
)

func init() {
	utilruntime.Must(configv1.InstallLegacy(nodeConfigScheme))

	// TODO: Remove this when we generate config.openshift.io/v1 configs
	utilruntime.Must(configv1.InstallLegacy(nodeConfigScheme))

	configCodecfactory := serializer.NewCodecFactory(nodeConfigScheme)
	nodeConfigDecoder = configCodecfactory.UniversalDecoder(configv1.LegacySchemeGroupVersion)
	nodeConfigEncoder = configCodecfactory.LegacyCodec(configv1.LegacySchemeGroupVersion)
}

func WriteNodeConfig(filename string, config *configv1.NodeConfig) error {
	json, err := runtime.Encode(nodeConfigEncoder, config)
	if err != nil {
		return fmt.Errorf("unable to encode node config: %v", err)
	}
	yamlBytes, err := yaml.JSONToYAML(json)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(filename, yamlBytes, 0644)
}

func ReadNodeConfig(filename string) (*configv1.NodeConfig, error) {
	masterBytes, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	obj, err := runtime.Decode(nodeConfigDecoder, masterBytes)
	if err != nil {
		return nil, err
	}
	nodeConfig, ok := obj.(*configv1.NodeConfig)
	if !ok {
		return nil, fmt.Errorf("object %T is not NodeConfig", nodeConfig)
	}
	return nodeConfig, nil
}
