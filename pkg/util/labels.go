package util

import (
	"fmt"
	"reflect"

	kmeta "k8s.io/kubernetes/pkg/api/meta"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/fielderrors"
)

// MergeInto flags
const (
	OverwriteExistingDstKey = 1 << iota
	ErrorOnExistingDstKey
	ErrorOnDifferentDstKeyValue
)

// ReportError reports the single item validation error and properly set the
// prefix and index to match the Config item JSON index
func ReportError(allErrs *fielderrors.ValidationErrorList, index int, err fielderrors.ValidationError) {
	i := fielderrors.ValidationErrorList{}
	*allErrs = append(*allErrs, append(i, &err).PrefixIndex(index).Prefix("item")...)
}

// AddObjectLabels adds new label(s) to a single runtime.Object
func AddObjectLabels(obj runtime.Object, labels labels.Set) error {
	if labels == nil {
		return nil
	}

	accessor, err := kmeta.Accessor(obj)

	if err != nil {
		if _, ok := obj.(*runtime.Unstructured); !ok {
			// error out if it's not possible to get an accessor and it's also not an unstructured object
			return err
		}
	} else {
		metaLabels := accessor.Labels()
		if metaLabels == nil {
			metaLabels = make(map[string]string)
		}

		if err := MergeInto(metaLabels, labels, ErrorOnDifferentDstKeyValue); err != nil {
			return fmt.Errorf("unable to add labels to Template.%s: %v", accessor.Kind(), err)
		}
		accessor.SetLabels(metaLabels)

		return nil
	}

	// handle unstructured object
	// TODO: allow meta.Accessor to handle runtime.Unstructured
	if unstruct, ok := obj.(*runtime.Unstructured); ok && unstruct.Object != nil {
		// the presence of "metadata" is sufficient for us to apply the rules for Kube-like
		// objects.
		// TODO: add swagger detection to allow this to happen more effectively
		if obj, ok := unstruct.Object["metadata"]; ok {
			if m, ok := obj.(map[string]interface{}); ok {

				existing := make(map[string]string)
				if l, ok := m["labels"]; ok {
					if found, ok := extractLabels(l); ok {
						existing = found
					}
				}
				if err := MergeInto(existing, labels, OverwriteExistingDstKey); err != nil {
					return err
				}
				m["labels"] = mapToGeneric(existing)
			}
			return nil
		}

		// only attempt to set root labels if a root object called labels exists
		// TODO: add swagger detection to allow this to happen more effectively
		if obj, ok := unstruct.Object["labels"]; ok {
			existing := make(map[string]string)
			if found, ok := extractLabels(obj); ok {
				existing = found
			}
			if err := MergeInto(existing, labels, OverwriteExistingDstKey); err != nil {
				return err
			}
			unstruct.Object["labels"] = mapToGeneric(existing)
			return nil
		}
	}

	return nil
}

// extractLabels extracts a map[string]string from a map[string]interface{}
func extractLabels(obj interface{}) (map[string]string, bool) {
	if obj == nil {
		return nil, false
	}
	lm, ok := obj.(map[string]interface{})
	if !ok {
		return nil, false
	}
	existing := make(map[string]string)
	for k, v := range lm {
		switch t := v.(type) {
		case string:
			existing[k] = t
		}
	}
	return existing, true
}

// mapToGeneric converts a map[string]string into a map[string]interface{}
func mapToGeneric(obj map[string]string) map[string]interface{} {
	if obj == nil {
		return nil
	}
	res := make(map[string]interface{})
	for k, v := range obj {
		res[k] = v
	}
	return res
}

// MergeInto merges items from a src map into a dst map.
// Returns an error when the maps are not of the same type.
// Flags:
// - ErrorOnExistingDstKey
//     When set: Return an error if any of the dst keys is already set.
// - ErrorOnDifferentDstKeyValue
//     When set: Return an error if any of the dst keys is already set
//               to a different value than src key.
// - OverwriteDstKey
//     When set: Overwrite existing dst key value with src key value.
func MergeInto(dst, src interface{}, flags int) error {
	dstVal := reflect.ValueOf(dst)
	srcVal := reflect.ValueOf(src)

	if dstVal.Kind() != reflect.Map {
		return fmt.Errorf("dst is not a valid map: %v", dstVal.Kind())
	}
	if srcVal.Kind() != reflect.Map {
		return fmt.Errorf("src is not a valid map: %v", srcVal.Kind())
	}
	if dstTyp, srcTyp := dstVal.Type(), srcVal.Type(); !dstTyp.AssignableTo(srcTyp) {
		return fmt.Errorf("type mismatch, can't assign '%v' to '%v'", srcTyp, dstTyp)
	}

	if dstVal.IsNil() {
		return fmt.Errorf("dst value is nil")
	}
	if srcVal.IsNil() {
		// Nothing to merge
		return nil
	}

	for _, k := range srcVal.MapKeys() {
		if dstVal.MapIndex(k).IsValid() {
			if flags&ErrorOnExistingDstKey != 0 {
				return fmt.Errorf("dst key already set (ErrorOnExistingDstKey=1), '%v'='%v'", k, dstVal.MapIndex(k))
			}
			if dstVal.MapIndex(k).String() != srcVal.MapIndex(k).String() {
				if flags&ErrorOnDifferentDstKeyValue != 0 {
					return fmt.Errorf("dst key already set to a different value (ErrorOnDifferentDstKeyValue=1), '%v'='%v'", k, dstVal.MapIndex(k))
				}
				if flags&OverwriteExistingDstKey != 0 {
					dstVal.SetMapIndex(k, srcVal.MapIndex(k))
				}
			}
		} else {
			dstVal.SetMapIndex(k, srcVal.MapIndex(k))
		}
	}

	return nil
}
