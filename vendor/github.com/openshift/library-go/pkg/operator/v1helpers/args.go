package v1helpers

import (
	"fmt"
	"sort"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// FlagsFromUnstructured process the unstructured arguments usually retrieved from an operator's configuration file under a specific key.
// There are only two supported/valid types for arguments, that is []sting and/or string.
// Passing a different type yield an error.
//
// Use ToFlagSlice function to get a slice of string flags.
func FlagsFromUnstructured(unstructuredArgs map[string]interface{}) (map[string][]string, error) {
	return flagsFromUnstructured(unstructuredArgs)
}

// ToFlagSlice transforms the provided arguments to a slice of string flags.
// A flag name is taken directly from the key and the value is simply attached.
// A flag is repeated iff it has more than one value.
func ToFlagSlice(args map[string][]string) []string {
	var keys []string
	for key := range args {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var flags []string
	for _, key := range keys {
		for _, token := range args[key] {
			flags = append(flags, fmt.Sprintf("--%s=%s", key, token))
		}
	}
	return flags
}

// flagsFromUnstructured process the unstructured arguments (interface{}) to a map of strings.
// There are only two supported/valid types for arguments, that is []sting and/or string.
// Passing a different type yield an error.
func flagsFromUnstructured(unstructuredArgs map[string]interface{}) (map[string][]string, error) {
	ret := map[string][]string{}
	for argName, argRawValue := range unstructuredArgs {
		var argsSlice []string
		var found bool
		var err error

		argsSlice, found, err = unstructured.NestedStringSlice(unstructuredArgs, argName)
		if !found || err != nil {
			str, found, err := unstructured.NestedString(unstructuredArgs, argName)
			if !found || err != nil {
				return nil, fmt.Errorf("unable to process an argument, incorrect value %v under %v key, expected []string or string", argRawValue, argName)
			}
			argsSlice = append(argsSlice, str)
		}

		ret[argName] = argsSlice
	}

	return ret, nil
}
