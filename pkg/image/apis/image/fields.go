package image

import "k8s.io/apimachinery/pkg/fields"

// ImageStreamToSelectableFields returns a label set that represents the object.
func ImageStreamToSelectableFields(ir *ImageStream) fields.Set {
	return fields.Set{
		"metadata.name":                ir.Name,
		"metadata.namespace":           ir.Namespace,
		"spec.dockerImageRepository":   ir.Spec.DockerImageRepository,
		"status.dockerImageRepository": ir.Status.DockerImageRepository,
	}
}
