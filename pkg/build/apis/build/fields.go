package build

import "k8s.io/apimachinery/pkg/fields"

// BuildToSelectableFields returns a label set that represents the object
// changes to the returned keys require registering conversions for existing versions using Scheme.AddFieldLabelConversionFunc
func BuildToSelectableFields(build *Build) fields.Set {
	return fields.Set{
		"metadata.name":      build.Name,
		"metadata.namespace": build.Namespace,
		"status":             string(build.Status.Phase),
		"podName":            GetBuildPodName(build),
	}
}
