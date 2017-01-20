package api

import (
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/registry/generic"
)

// ImageToSelectableFields returns a label set that represents the object.
func ImageToSelectableFields(image *Image) fields.Set {
	return generic.ObjectMetaFieldsSet(&image.ObjectMeta, true)
}

// ImageStreamToSelectableFields returns a label set that represents the object.
func ImageStreamToSelectableFields(ir *ImageStream) fields.Set {
	return generic.AddObjectMetaFieldsSet(fields.Set{
		"spec.dockerImageRepository":   ir.Spec.DockerImageRepository,
		"status.dockerImageRepository": ir.Status.DockerImageRepository,
	}, &ir.ObjectMeta, true)
}
