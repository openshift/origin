package util

import (
	"fmt"
	"reflect"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kmeta "github.com/GoogleCloudPlatform/kubernetes/pkg/api/meta"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util/fielderrors"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
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

// addReplicationControllerNestedLabels adds new label(s) to a nested labels of a single ReplicationController object
func addReplicationControllerNestedLabels(obj *kapi.ReplicationController, labels labels.Set) error {
	if obj.Spec.Template.Labels == nil {
		obj.Spec.Template.Labels = make(map[string]string)
	}
	if err := MergeInto(obj.Spec.Template.Labels, labels, ErrorOnDifferentDstKeyValue); err != nil {
		return fmt.Errorf("unable to add labels to Template.ReplicationController.Spec.Template: %v", err)
	}
	if err := MergeInto(obj.Spec.Template.Labels, obj.Spec.Selector, ErrorOnDifferentDstKeyValue); err != nil {
		return fmt.Errorf("unable to add labels to Template.ReplicationController.Spec.Template: %v", err)
	}
	// Selector and Spec.Template.Labels must be equal
	if obj.Spec.Selector == nil {
		obj.Spec.Selector = make(map[string]string)
	}
	if err := MergeInto(obj.Spec.Selector, obj.Spec.Template.Labels, ErrorOnDifferentDstKeyValue); err != nil {
		return fmt.Errorf("unable to add labels to Template.ReplicationController.Spec.Selector: %v", err)
	}
	return nil
}

// addDeploymentConfigNestedLabels adds new label(s) to a nested labels of a single DeploymentConfig object
func addDeploymentConfigNestedLabels(obj *deployapi.DeploymentConfig, labels labels.Set) error {
	if obj.Template.ControllerTemplate.Template.Labels == nil {
		obj.Template.ControllerTemplate.Template.Labels = make(map[string]string)
	}
	if err := MergeInto(obj.Template.ControllerTemplate.Template.Labels, labels, ErrorOnDifferentDstKeyValue); err != nil {
		return fmt.Errorf("unable to add labels to Template.DeploymentConfig.Template.ControllerTemplate.Template: %v", err)
	}
	return nil
}

// AddObjectLabels adds new label(s) to a single runtime.Object
func AddObjectLabels(obj runtime.Object, labels labels.Set) error {
	if labels == nil {
		// Nothing to add
		return nil
	}

	accessor, err := kmeta.Accessor(obj)
	if err != nil {
		return err
	}

	metaLabels := accessor.Labels()
	if metaLabels == nil {
		metaLabels = make(map[string]string)
	}

	if err := MergeInto(metaLabels, labels, ErrorOnDifferentDstKeyValue); err != nil {
		return fmt.Errorf("unable to add labels to Template.%s: %v", accessor.Kind(), err)
	}
	accessor.SetLabels(metaLabels)

	// Handle nested Labels
	switch objType := obj.(type) {
	case *kapi.ReplicationController:
		return addReplicationControllerNestedLabels(objType, labels)
	case *deployapi.DeploymentConfig:
		return addDeploymentConfigNestedLabels(objType, labels)
	}

	return nil
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
