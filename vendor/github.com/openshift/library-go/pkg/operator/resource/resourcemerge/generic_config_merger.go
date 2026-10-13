package resourcemerge

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"k8s.io/klog/v2"
	"sigs.k8s.io/yaml"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"
)

// MergeConfigMap takes a configmap, the target key, special overlay funcs a list of config configs to overlay on top of each other
// It returns the resultant configmap and a bool indicating if any changes were made to the configmap
func MergeConfigMap(configMap *corev1.ConfigMap, configKey string, specialCases map[string]MergeFunc, configYAMLs ...[]byte) (*corev1.ConfigMap, bool, error) {
	return MergePrunedConfigMap(nil, configMap, configKey, specialCases, configYAMLs...)
}

// MergePrunedConfigMap takes a configmap, the target key, special overlay funcs a list of config configs to overlay on top of each other
// It returns the resultant configmap and a bool indicating if any changes were made to the configmap.
// It roundtrips the config through the given schema.
func MergePrunedConfigMap(schema runtime.Object, configMap *corev1.ConfigMap, configKey string, specialCases map[string]MergeFunc, configYAMLs ...[]byte) (*corev1.ConfigMap, bool, error) {
	configBytes, err := MergePrunedProcessConfig(schema, specialCases, configYAMLs...)
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
			klog.Warning(err)
			// maybe it's just json
			prevConfigJSON = currentConfigYAML
		}
		prevConfig := map[string]interface{}{}
		if err := json.NewDecoder(bytes.NewBuffer(prevConfigJSON)).Decode(&prevConfig); err != nil {
			return nil, err
		}

		if len(currConfigYAML) > 0 {
			currConfigJSON, err := kyaml.ToJSON(currConfigYAML)
			if err != nil {
				klog.Warning(err)
				// maybe it's just json
				currConfigJSON = currConfigYAML
			}
			currConfig := map[string]interface{}{}
			if err := json.NewDecoder(bytes.NewBuffer(currConfigJSON)).Decode(&currConfig); err != nil {
				return nil, err
			}

			// protected against mismatched typemeta
			prevAPIVersion, _, _ := unstructured.NestedString(prevConfig, "apiVersion")
			prevKind, _, _ := unstructured.NestedString(prevConfig, "kind")
			currAPIVersion, _, _ := unstructured.NestedString(currConfig, "apiVersion")
			currKind, _, _ := unstructured.NestedString(currConfig, "kind")
			currGVKSet := len(currAPIVersion) > 0 || len(currKind) > 0
			gvkMismatched := currAPIVersion != prevAPIVersion || currKind != prevKind
			if currGVKSet && gvkMismatched {
				return nil, fmt.Errorf("%v/%v does not equal %v/%v", currAPIVersion, currKind, prevAPIVersion, prevKind)
			}

			if err := mergeConfig(prevConfig, currConfig, "", specialCases); err != nil {
				return nil, err
			}
		}

		currentConfigYAML, err = runtime.Encode(unstructured.UnstructuredJSONScheme, &unstructured.Unstructured{Object: prevConfig})
		if err != nil {
			return nil, err
		}
	}

	return currentConfigYAML, nil
}

// MergePrunedProcessConfig merges a series of config yaml files together with each later one overlaying all previous.
// The result is roundtripped through the given schema if it is non-nil.
func MergePrunedProcessConfig(schema runtime.Object, specialCases map[string]MergeFunc, configYAMLs ...[]byte) ([]byte, error) {
	bs, err := MergeProcessConfig(specialCases, configYAMLs...)
	if err != nil {
		return nil, err
	}

	if schema == nil {
		return bs, nil
	}

	// roundtrip through the schema
	typed := schema.DeepCopyObject()
	if err := yaml.Unmarshal(bs, typed); err != nil {
		return nil, err
	}
	typedBytes, err := json.Marshal(typed)
	if err != nil {
		return nil, err
	}
	var untypedJSON map[string]interface{}
	if err := json.Unmarshal(typedBytes, &untypedJSON); err != nil {
		return nil, err
	}

	// and intersect output with input because we cannot rely on omitempty in the schema
	inputBytes, err := yaml.YAMLToJSON(bs)
	if err != nil {
		return nil, err
	}
	var inputJSON map[string]interface{}
	if err := json.Unmarshal(inputBytes, &inputJSON); err != nil {
		return nil, err
	}
	return json.Marshal(intersectJSON(inputJSON, untypedJSON))
}

type MergeFunc func(dst, src interface{}, currentPath string) (interface{}, error)

var _ MergeFunc = RemoveConfig

// RemoveConfig is a merge func that elimintes an entire path from the config
func RemoveConfig(dst, src interface{}, currentPath string) (interface{}, error) {
	return dst, nil
}

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

// jsonIntersection returns the intersection of both JSON object,
// preferring the values of the first argument.
func intersectJSON(x1, x2 map[string]interface{}) map[string]interface{} {
	if x1 == nil || x2 == nil {
		return nil
	}
	ret := map[string]interface{}{}
	for k, v1 := range x1 {
		v2, ok := x2[k]
		if !ok {
			continue
		}
		ret[k] = intersectValue(v1, v2)
	}
	return ret
}

func intersectArray(x1, x2 []interface{}) []interface{} {
	if x1 == nil || x2 == nil {
		return nil
	}
	ret := make([]interface{}, 0, len(x1))
	for i := range x1 {
		if i >= len(x2) {
			break
		}
		ret = append(ret, intersectValue(x1[i], x2[i]))
	}
	return ret
}

func intersectValue(x1, x2 interface{}) interface{} {
	switch x1 := x1.(type) {
	case map[string]interface{}:
		x2, ok := x2.(map[string]interface{})
		if !ok {
			return x1
		}
		return intersectJSON(x1, x2)
	case []interface{}:
		x2, ok := x2.([]interface{})
		if !ok {
			return x1
		}
		return intersectArray(x1, x2)
	default:
		return x1
	}
}

// IsRequiredConfigPresent can check an observedConfig to see if certain required paths are present in that config.
// This allows operators to require certain configuration to be observed before proceeding to honor a configuration or roll it out.
func IsRequiredConfigPresent(config []byte, requiredPaths [][]string) error {
	if len(config) == 0 {
		return fmt.Errorf("no observedConfig")
	}

	existingConfig := map[string]interface{}{}
	if err := json.NewDecoder(bytes.NewBuffer(config)).Decode(&existingConfig); err != nil {
		return fmt.Errorf("error parsing config, %v", err)
	}

	for _, requiredPath := range requiredPaths {
		configVal, found, err := unstructured.NestedFieldNoCopy(existingConfig, requiredPath...)
		if err != nil {
			return fmt.Errorf("error reading %v from config, %v", strings.Join(requiredPath, "."), err)
		}
		if !found {
			return fmt.Errorf("%v missing from config", strings.Join(requiredPath, "."))
		}
		if configVal == nil {
			return fmt.Errorf("%v null in config", strings.Join(requiredPath, "."))
		}
		if configValSlice, ok := configVal.([]interface{}); ok && len(configValSlice) == 0 {
			return fmt.Errorf("%v empty in config", strings.Join(requiredPath, "."))
		}
		if configValString, ok := configVal.(string); ok && len(configValString) == 0 {
			return fmt.Errorf("%v empty in config", strings.Join(requiredPath, "."))
		}
	}
	return nil
}
