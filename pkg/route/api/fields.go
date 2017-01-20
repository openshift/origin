package api

import (
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/registry/generic"
)

// RouteToSelectableFields returns a label set that represents the object
func RouteToSelectableFields(route *Route) fields.Set {
	return generic.AddObjectMetaFieldsSet(fields.Set{
		"spec.path":    route.Spec.Path,
		"spec.host":    route.Spec.Host,
		"spec.to.name": route.Spec.To.Name,
	}, &route.ObjectMeta, true)
}
