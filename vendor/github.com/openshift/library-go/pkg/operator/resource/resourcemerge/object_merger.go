package resourcemerge

import (
	"reflect"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// EnsureObjectMeta writes namespace, name, labels, and annotations.  Don't set other things here.
// TODO finalizer support maybe?
func EnsureObjectMeta(modified *bool, existing *metav1.ObjectMeta, required metav1.ObjectMeta) {
	SetStringIfSet(modified, &existing.Namespace, required.Namespace)
	SetStringIfSet(modified, &existing.Name, required.Name)
	MergeMap(modified, &existing.Labels, required.Labels)
	MergeMap(modified, &existing.Annotations, required.Annotations)
	MergeOwnerRefs(modified, &existing.OwnerReferences, required.OwnerReferences)
}

// WithCleanLabelsAndAnnotations cleans the metadata off the removal annotations/labels/ownerrefs
// (those that end with trailing "-")
func WithCleanLabelsAndAnnotations(obj metav1.Object) metav1.Object {
	obj.SetAnnotations(cleanRemovalKeys(obj.GetAnnotations()))
	obj.SetLabels(cleanRemovalKeys(obj.GetLabels()))
	obj.SetOwnerReferences(cleanRemovalOwnerRefs(obj.GetOwnerReferences()))
	return obj
}

func cleanRemovalKeys(required map[string]string) map[string]string {
	for k := range required {
		if strings.HasSuffix(k, "-") {
			delete(required, k)
		}
	}
	return required
}

func stringPtr(val string) *string {
	return &val
}

func SetString(modified *bool, existing *string, required string) {
	if required != *existing {
		*existing = required
		*modified = true
	}
}

func SetStringIfSet(modified *bool, existing *string, required string) {
	if len(required) == 0 {
		return
	}
	if required != *existing {
		*existing = required
		*modified = true
	}
}

func setStringPtr(modified *bool, existing **string, required *string) {
	if *existing == nil || (required == nil && *existing != nil) {
		*modified = true
		*existing = required
		return
	}
	SetString(modified, *existing, *required)
}

func SetStringSlice(modified *bool, existing *[]string, required []string) {
	if !reflect.DeepEqual(required, *existing) {
		*existing = required
		*modified = true
	}
}

func SetStringSliceIfSet(modified *bool, existing *[]string, required []string) {
	if required == nil {
		return
	}
	if !reflect.DeepEqual(required, *existing) {
		*existing = required
		*modified = true
	}
}

func BoolPtr(val bool) *bool {
	return &val
}

func SetBool(modified *bool, existing *bool, required bool) {
	if required != *existing {
		*existing = required
		*modified = true
	}
}

func setBoolPtr(modified *bool, existing **bool, required *bool) {
	if *existing == nil || (required == nil && *existing != nil) {
		*modified = true
		*existing = required
		return
	}
	SetBool(modified, *existing, *required)
}

func int64Ptr(val int64) *int64 {
	return &val
}

func SetInt32(modified *bool, existing *int32, required int32) {
	if required != *existing {
		*existing = required
		*modified = true
	}
}

func SetInt32IfSet(modified *bool, existing *int32, required int32) {
	if required == 0 {
		return
	}

	SetInt32(modified, existing, required)
}

func SetInt64(modified *bool, existing *int64, required int64) {
	if required != *existing {
		*existing = required
		*modified = true
	}
}

func setInt64Ptr(modified *bool, existing **int64, required *int64) {
	if *existing == nil || (required == nil && *existing != nil) {
		*modified = true
		*existing = required
		return
	}
	SetInt64(modified, *existing, *required)
}

func MergeMap(modified *bool, existing *map[string]string, required map[string]string) {
	if *existing == nil {
		*existing = map[string]string{}
	}
	for k, v := range required {
		actualKey := k
		removeKey := false

		// if "required" map contains a key with "-" as suffix, remove that
		// key from the existing map instead of replacing the value
		if strings.HasSuffix(k, "-") {
			removeKey = true
			actualKey = strings.TrimRight(k, "-")
		}

		if existingV, ok := (*existing)[actualKey]; removeKey {
			if !ok {
				continue
			}
			// value found -> it should be removed
			delete(*existing, actualKey)
			*modified = true

		} else if !ok || v != existingV {
			*modified = true
			(*existing)[actualKey] = v
		}
	}
}

func SetMapStringString(modified *bool, existing *map[string]string, required map[string]string) {
	if *existing == nil {
		*existing = map[string]string{}
	}

	if !reflect.DeepEqual(*existing, required) {
		*existing = required
	}
}

func SetMapStringStringIfSet(modified *bool, existing *map[string]string, required map[string]string) {
	if required == nil {
		return
	}
	if *existing == nil {
		*existing = map[string]string{}
	}

	if !reflect.DeepEqual(*existing, required) {
		*existing = required
	}
}

func MergeOwnerRefs(modified *bool, existing *[]metav1.OwnerReference, required []metav1.OwnerReference) {
	if *existing == nil {
		*existing = []metav1.OwnerReference{}
	}

	for _, o := range required {
		removeOwner := false

		// if "required" ownerRefs contain an owner.UID with "-" as suffix, remove that
		// ownerRef from the existing ownerRefs instead of replacing the value
		// NOTE: this is the same format as kubectl annotate and kubectl label
		if strings.HasSuffix(string(o.UID), "-") {
			removeOwner = true
		}

		existedIndex := 0

		for existedIndex < len(*existing) {
			if ownerRefMatched(o, (*existing)[existedIndex]) {
				break
			}
			existedIndex++
		}

		if existedIndex == len(*existing) {
			// There is no matched ownerref found, append the ownerref
			// if it is not to be removed.
			if !removeOwner {
				*existing = append(*existing, o)
				*modified = true
			}
			continue
		}

		if removeOwner {
			*existing = append((*existing)[:existedIndex], (*existing)[existedIndex+1:]...)
			*modified = true
			continue
		}

		if !reflect.DeepEqual(o, (*existing)[existedIndex]) {
			(*existing)[existedIndex] = o
			*modified = true
		}
	}
}

func ownerRefMatched(existing, required metav1.OwnerReference) bool {
	if existing.Name != required.Name {
		return false
	}

	if existing.Kind != required.Kind {
		return false
	}

	existingGV, err := schema.ParseGroupVersion(existing.APIVersion)

	if err != nil {
		return false
	}

	requiredGV, err := schema.ParseGroupVersion(required.APIVersion)

	if err != nil {
		return false
	}

	if existingGV.Group != requiredGV.Group {
		return false
	}

	return true
}

func cleanRemovalOwnerRefs(required []metav1.OwnerReference) []metav1.OwnerReference {
	for k := 0; k < len(required); k++ {
		if strings.HasSuffix(string(required[k].UID), "-") {
			required = append(required[:k], required[k+1:]...)
			k--
		}
	}
	return required
}
