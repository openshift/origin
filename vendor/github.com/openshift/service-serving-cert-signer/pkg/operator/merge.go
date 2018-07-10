package operator

import (
	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"
)

func mergeProcessConfig(defaultConfigYAML, userConfigYAML []byte, specialCases map[string]mergeFunc) ([]byte, error) {
	defaultConfigJSON, err := kyaml.ToJSON(defaultConfigYAML)
	if err != nil {
		return nil, err
	}
	defaultConfigObj, err := runtime.Decode(unstructured.UnstructuredJSONScheme, defaultConfigJSON)
	if err != nil {
		return nil, err
	}
	defaultConfig := defaultConfigObj.(*unstructured.Unstructured)

	if len(userConfigYAML) > 0 {
		userConfigJSON, err := kyaml.ToJSON(userConfigYAML)
		if err != nil {
			glog.Warning(err)
			// maybe it's just yaml
			userConfigJSON = userConfigYAML
		}
		userConfigObj, err := runtime.Decode(unstructured.UnstructuredJSONScheme, userConfigJSON)
		if err != nil {
			return nil, err
		}
		userConfig := userConfigObj.(*unstructured.Unstructured)
		if err := mergeConfig(defaultConfig.Object, userConfig.Object, "", specialCases); err != nil {
			return nil, err
		}
	}

	configBytes, err := runtime.Encode(unstructured.UnstructuredJSONScheme, defaultConfig)
	if err != nil {
		return nil, err
	}
	return configBytes, nil
}

type mergeFunc func(dst, src interface{}, currentPath string) (interface{}, error)

// mergeConfig overwrites entries in curr by additional.  It modifies curr.
func mergeConfig(curr, additional map[string]interface{}, currentPath string, specialCases map[string]mergeFunc) error {
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
