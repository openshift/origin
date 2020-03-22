package configobserver

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

// Pruned returns the unstructured filtered by the given paths, i.e. everything
// outside of them will be dropped. The returned data structure might overlap
// with the input, but the input is not mutated. In case of error for a path,
// that path is dropped.
func Pruned(obj map[string]interface{}, pths ...[]string) map[string]interface{} {
	if obj == nil || len(pths) == 0 {
		return obj
	}

	ret := map[string]interface{}{}
	if len(pths) == 1 {
		x, found, err := unstructured.NestedFieldCopy(obj, pths[0]...)
		if err != nil || !found {
			return ret
		}
		unstructured.SetNestedField(ret, x, pths[0]...)
		return ret
	}

	for i, p := range pths {
		x, found, err := unstructured.NestedFieldCopy(obj, p...)
		if err != nil {
			continue
		}
		if !found {
			continue
		}
		if i < len(pths)-1 {
			// this might be overwritten by a later path
			x = runtime.DeepCopyJSONValue(x)
		}
		if err := unstructured.SetNestedField(ret, x, p...); err != nil {
			continue
		}
	}

	return ret
}
