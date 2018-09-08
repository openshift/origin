package componentinstall

import (
	"fmt"
	"io/ioutil"

	"github.com/ghodss/yaml"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	legacyconfigv1 "github.com/openshift/api/legacyconfig/v1"
	legacyconfigv1conversions "github.com/openshift/origin/pkg/cmd/server/apis/config/v1"
)

var (
	masterConfigScheme  = runtime.NewScheme()
	masterConfigDecoder runtime.Decoder
	masterConfigEncoder runtime.Encoder
)

func init() {
	// TODO: Remove this when we start generating config.openshift.io/v1 configs
	utilruntime.Must(legacyconfigv1conversions.InstallLegacy(masterConfigScheme))

	configCodecfactory := serializer.NewCodecFactory(masterConfigScheme)
	masterConfigDecoder = configCodecfactory.UniversalDecoder(legacyconfigv1.LegacySchemeGroupVersion)
	masterConfigEncoder = configCodecfactory.LegacyCodec(legacyconfigv1.LegacySchemeGroupVersion)
}

func WriteMasterConfig(filename string, config *legacyconfigv1.MasterConfig) error {
	json, err := runtime.Encode(masterConfigEncoder, config)
	if err != nil {
		return fmt.Errorf("unable to encode master config: %v", err)
	}
	yamlBytes, err := yaml.JSONToYAML(json)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(filename, yamlBytes, 0644)
}

func ReadMasterConfig(filename string) (*legacyconfigv1.MasterConfig, error) {
	masterBytes, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return ReadMasterConfigBytes(masterBytes)
}

func ReadMasterConfigBytes(masterBytes []byte) (*legacyconfigv1.MasterConfig, error) {
	obj, err := runtime.Decode(masterConfigDecoder, masterBytes)
	if err != nil {
		return nil, err
	}
	masterConfig, ok := obj.(*legacyconfigv1.MasterConfig)
	if !ok {
		return nil, fmt.Errorf("object %T is not MasterConfig", masterConfig)
	}
	return masterConfig, nil
}
