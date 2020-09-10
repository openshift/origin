package main

import (
	"fmt"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/kubernetes/openshift-hack/e2e/annotate"

	_ "github.com/openshift/origin/test/extended"
)

// mergeMaps updates an existing map of string slices with the
// contents of a new map. Duplicate keys are allowed but duplicate
// values are not to ensure matches are defined in this repo or
// openshift/kubernetes but not both.
func mergeMaps(existingMap, newMap map[string][]string) error {
	for key, newValues := range newMap {
		if _, ok := existingMap[key]; !ok {
			existingMap[key] = []string{}
		}
		existingValues := sets.NewString(existingMap[key]...)
		for _, value := range newValues {
			if existingValues.Has(value) {
				return fmt.Errorf("value %s for key %s is already present", value, key)
			}
			existingMap[key] = append(existingMap[key], value)
		}
	}
	return nil
}

func init() {
	// Merge the local rules with the rules for the kube e2e tests
	// inherited from openshift/kubernetes.
	err := mergeMaps(annotate.TestMaps, testMaps)
	if err != nil {
		panic(fmt.Sprintf("Error updating annotate.TestMaps: %v", err))
	}
	err = mergeMaps(annotate.LabelExcludes, labelExcludes)
	if err != nil {
		panic(fmt.Sprintf("Error updating annotate.LabelExcludes: %v", err))
	}
}

func main() {
	annotate.Run()
}
