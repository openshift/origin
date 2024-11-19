package common

import (
	"fmt"

	"encoding/json"

	corev1 "k8s.io/api/core/v1"
)

// Images contain data derived from what github.com/openshift/installer's
// bootkube.sh provides.  If you want to add a new image, you need
// to "ratchet" the change as follows:
//
// Add the image here and also a CLI option with a default value
// Change the installer to pass that arg with the image from the CVO
// (some time later) Change the option to required and drop the default
type Images struct {
	ReleaseVersion string `json:"releaseVersion,omitempty"`
	RenderConfigImages
	ControllerConfigImages
}

// RenderConfigImages are image names used to render templates under ./manifests/
type RenderConfigImages struct {
	MachineConfigOperator string `json:"machineConfigOperator"`
	// The new format image
	BaseOSContainerImage string `json:"baseOSContainerImage"`
	// The matching extensions container for the new format image
	BaseOSExtensionsContainerImage string `json:"baseOSExtensionsContainerImage"`
	// These have to be named differently from the ones in ControllerConfigImages
	// or we get errors about ambiguous selectors because both structs are
	// combined in the Images struct.
	KeepalivedBootstrap          string `json:"keepalived"`
	CorednsBootstrap             string `json:"coredns"`
	BaremetalRuntimeCfgBootstrap string `json:"baremetalRuntimeCfg"`
	OauthProxy                   string `json:"oauthProxy"`
	KubeRbacProxy                string `json:"kubeRbacProxy"`
}

// ControllerConfigImages are image names used to render templates under ./templates/
type ControllerConfigImages struct {
	InfraImage          string `json:"infraImage"`
	Keepalived          string `json:"keepalivedImage"`
	Coredns             string `json:"corednsImage"`
	Haproxy             string `json:"haproxyImage"`
	BaremetalRuntimeCfg string `json:"baremetalRuntimeCfgImage"`
}

// Parses the JSON blob containing the images information into an Images struct.
func ParseImagesFromBytes(in []byte) (*Images, error) {
	img := &Images{}

	if err := json.Unmarshal(in, img); err != nil {
		return nil, fmt.Errorf("could not parse images.json bytes: %w", err)
	}

	return img, nil
}

// Reads the contents of the provided ConfigMap into an Images struct.
func ParseImagesFromConfigMap(cm *corev1.ConfigMap) (*Images, error) {
	if err := validateMCOConfigMap(cm, MachineConfigOperatorImagesConfigMapName, []string{"images.json"}, nil); err != nil {
		return nil, err
	}

	return ParseImagesFromBytes([]byte(cm.Data["images.json"]))
}

// Holds the contents of the machine-config-osimageurl ConfigMap.
type OSImageURLConfig struct {
	BaseOSContainerImage           string
	BaseOSExtensionsContainerImage string
	OSImageURL                     string
	ReleaseVersion                 string
}

// Reads the contents of the provided ConfigMap into an OSImageURLConfig struct.
func ParseOSImageURLConfigMap(cm *corev1.ConfigMap) (*OSImageURLConfig, error) {
	reqKeys := []string{"baseOSContainerImage", "baseOSExtensionsContainerImage", "osImageURL", "releaseVersion"}

	if err := validateMCOConfigMap(cm, MachineConfigOSImageURLConfigMapName, reqKeys, nil); err != nil {
		return nil, err
	}

	return &OSImageURLConfig{
		BaseOSContainerImage:           cm.Data["baseOSContainerImage"],
		BaseOSExtensionsContainerImage: cm.Data["baseOSExtensionsContainerImage"],
		OSImageURL:                     cm.Data["osImageURL"],
		ReleaseVersion:                 cm.Data["releaseVersion"],
	}, nil
}

// Validates a given ConfigMap in the MCO namespace. Valid in this case means the following:
// 1. The name matches what was provided.
// 2. The namespace is set to the MCO's namespace.
// 3. The data field has all of the expected keys.
// 4. The BinarayData field has all of the expected keys.
func validateMCOConfigMap(cm *corev1.ConfigMap, name string, reqDataKeys, reqBinaryKeys []string) error {
	if cm.Name != name {
		return fmt.Errorf("invalid ConfigMap, expected %s", name)
	}

	if cm.Namespace != MCONamespace {
		return fmt.Errorf("invalid namespace, expected %s", MCONamespace)
	}

	if reqDataKeys != nil {
		for _, reqKey := range reqDataKeys {
			if _, ok := cm.Data[reqKey]; !ok {
				return fmt.Errorf("expected missing data key %q to be present in ConfigMap %s", reqKey, cm.Name)
			}
		}
	}

	if reqBinaryKeys != nil {
		for _, reqKey := range reqBinaryKeys {
			if _, ok := cm.BinaryData[reqKey]; !ok {
				return fmt.Errorf("expecting missing binary data key %s to be present in ConfigMap %s", reqKey, cm.Name)
			}
		}
	}

	return nil
}
