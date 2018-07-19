package resourcemerge

import (
	"github.com/golang/glog"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"

	"reflect"
)

// MergeConfigMap takes a configmap, the target key, special overlay funcs a list of config configs to overlay on top of each other
// It returns the resultant configmap and a bool indicating if any changes were made to the configmap
func MergeConfigMap(configMap *corev1.ConfigMap, configKey string, specialCases map[string]MergeFunc, configYAMLs ...[]byte) (*corev1.ConfigMap, bool, error) {
	configBytes, err := MergeProcessConfig(specialCases, configYAMLs...)
	if err != nil {
		return nil, false, err
	}

	if reflect.DeepEqual(configMap.Data[configKey], configBytes) {
		return configMap, false, nil
	}

	ret := configMap.DeepCopy()
	ret.Data[configKey] = string(configBytes)

	return ret, true, nil
}

// MergeProcessConfig merges a series of config yaml files together with each later one overlaying all previous
func MergeProcessConfig(specialCases map[string]MergeFunc, configYAMLs ...[]byte) ([]byte, error) {
	currentConfigYAML := configYAMLs[0]

	for _, currConfigYAML := range configYAMLs[1:] {
		prevConfigJSON, err := kyaml.ToJSON(currentConfigYAML)
		if err != nil {
			glog.Warning(err)
			// maybe it's just json
			prevConfigJSON = currentConfigYAML
		}
		prevConfigObj, err := runtime.Decode(unstructured.UnstructuredJSONScheme, prevConfigJSON)
		if err != nil {
			return nil, err
		}
		prevConfig := prevConfigObj.(*unstructured.Unstructured)

		if len(currConfigYAML) > 0 {
			currConfigJSON, err := kyaml.ToJSON(currConfigYAML)
			if err != nil {
				glog.Warning(err)
				// maybe it's just json
				currConfigJSON = currConfigYAML
			}
			currConfigObj, err := runtime.Decode(unstructured.UnstructuredJSONScheme, currConfigJSON)
			if err != nil {
				return nil, err
			}
			currConfig := currConfigObj.(*unstructured.Unstructured)
			if err := mergeConfig(prevConfig.Object, currConfig.Object, "", specialCases); err != nil {
				return nil, err
			}
		}

		currentConfigYAML, err = runtime.Encode(unstructured.UnstructuredJSONScheme, prevConfig)
		if err != nil {
			return nil, err
		}
	}

	return currentConfigYAML, nil
}

type MergeFunc func(dst, src interface{}, currentPath string) (interface{}, error)

// mergeConfig overwrites entries in curr by additional.  It modifies curr.
func mergeConfig(curr, additional map[string]interface{}, currentPath string, specialCases map[string]MergeFunc) error {
	for additionalKey, additionalVal := range additional {
		fullKey := currentPath + "." + additionalKey
		specialCase, ok := specialCases[fullKey]
		if ok {
			var err error
			curr[additionalKey], err = specialCase(curr[additionalKey], additionalVal, currentPath)
			if err != nil {
				return err
			}
			continue
		}

		currVal, ok := curr[additionalKey]
		if !ok {
			curr[additionalKey] = additionalVal
			continue
		}

		// only some scalars are accepted
		switch castVal := additionalVal.(type) {
		case map[string]interface{}:
			currValAsMap, ok := currVal.(map[string]interface{})
			if !ok {
				currValAsMap = map[string]interface{}{}
				curr[additionalKey] = currValAsMap
			}

			err := mergeConfig(currValAsMap, castVal, fullKey, specialCases)
			if err != nil {
				return err
			}
			continue

		default:
			if err := unstructured.SetNestedField(curr, castVal, additionalKey); err != nil {
				return err
			}
		}

	}

	return nil
}
