package resourceapply

import (
	"fmt"

	patch "github.com/evanphx/json-patch"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

// JSONPatch generates a JSON patch between original and modified objects and return the JSON as a string.
// In case of error, the returned string will contain the error messages.
func JSONPatch(original, modified runtime.Object) string {
	if original == nil {
		return "original object is nil"
	}
	if modified == nil {
		return "modified object is nil"
	}
	originalJSON, err := runtime.Encode(unstructured.UnstructuredJSONScheme, original)
	if err != nil {
		return fmt.Sprintf("unable to decode original to JSON: %v", err)
	}
	modifiedJSON, err := runtime.Encode(unstructured.UnstructuredJSONScheme, modified)
	if err != nil {
		return fmt.Sprintf("unable to decode modified to JSON: %v", err)
	}
	patchBytes, err := patch.CreateMergePatch(originalJSON, modifiedJSON)
	if err != nil {
		return fmt.Sprintf("unable to create JSON patch: %v", err)
	}
	return string(patchBytes)
}
