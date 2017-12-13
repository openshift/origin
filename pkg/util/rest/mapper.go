package rest

import (
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
)

// DefaultMultiRESTMapper returns the multi REST mapper with all OpenShift and
// Kubernetes objects already registered.
func DefaultMultiRESTMapper() meta.MultiRESTMapper {
	var restMapper meta.MultiRESTMapper
	seenGroups := sets.String{}
	for _, gv := range legacyscheme.Registry.EnabledVersions() {
		if seenGroups.Has(gv.Group) {
			continue
		}
		seenGroups.Insert(gv.Group)
		groupMeta, err := legacyscheme.Registry.Group(gv.Group)
		if err != nil {
			continue
		}
		restMapper = meta.MultiRESTMapper(append(restMapper, groupMeta.RESTMapper))
	}
	return restMapper
}
