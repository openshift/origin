package api

import (
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/registry/generic"
)

// BuildToSelectableFields returns a label set that represents the object
// changes to the returned keys require registering conversions for existing versions using Scheme.AddFieldLabelConversionFunc
func BuildToSelectableFields(build *Build) fields.Set {
	return generic.AddObjectMetaFieldsSet(fields.Set{
		"status":  string(build.Status.Phase),
		"podName": GetBuildPodName(build),
	}, &build.ObjectMeta, true)
}

// BuildConfigToSelectableFields returns a label set that represents the object
// changes to the returned keys require registering conversions for existing versions using Scheme.AddFieldLabelConversionFunc
func BuildConfigToSelectableFields(buildConfig *BuildConfig) fields.Set {
	return generic.ObjectMetaFieldsSet(&buildConfig.ObjectMeta, true)
}
