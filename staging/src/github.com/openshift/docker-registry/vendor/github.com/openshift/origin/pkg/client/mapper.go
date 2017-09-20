package client

import (
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/util/sets"
	kapi "k8s.io/kubernetes/pkg/api"
)

// DefaultMultiRESTMapper returns the multi REST mapper with all OpenShift and
// Kubernetes objects already registered.
func DefaultMultiRESTMapper() meta.MultiRESTMapper {
	var restMapper meta.MultiRESTMapper
	seenGroups := sets.String{}
	for _, gv := range kapi.Registry.EnabledVersions() {
		if seenGroups.Has(gv.Group) {
			continue
		}
		seenGroups.Insert(gv.Group)
		groupMeta, err := kapi.Registry.Group(gv.Group)
		if err != nil {
			continue
		}
		restMapper = meta.MultiRESTMapper(append(restMapper, groupMeta.RESTMapper))
	}
	return restMapper
}
