package tls

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

// gvr creates a schema.GroupVersionResource with the given fields.
// All fields must be non-empty.
func gvr(group, version, resource string) schema.GroupVersionResource {
	if group == "" {
		panic("gvr: group cannot be empty")
	}
	if version == "" {
		panic("gvr: version cannot be empty")
	}
	if resource == "" {
		panic("gvr: resource cannot be empty")
	}
	return schema.GroupVersionResource{Group: group, Version: version, Resource: resource}
}

// newObservedConfigTarget creates an observedConfigTarget with all required fields.
// This constructor ensures no fields are accidentally omitted when adding new entries.
// All string parameters and servingInfoPath elements must be non-empty.
func newObservedConfigTarget(
	namespace string,
	operatorConfigGVR schema.GroupVersionResource,
	operatorConfigName string,
	servingInfoPath []string,
	managementClusterComponent bool,
) observedConfigTarget {
	// Validate all string fields are non-empty
	if namespace == "" {
		panic("observedConfigTarget: namespace cannot be empty")
	}
	if operatorConfigGVR.Group == "" {
		panic("observedConfigTarget: operatorConfigGVR.Group cannot be empty")
	}
	if operatorConfigGVR.Version == "" {
		panic("observedConfigTarget: operatorConfigGVR.Version cannot be empty")
	}
	if operatorConfigGVR.Resource == "" {
		panic("observedConfigTarget: operatorConfigGVR.Resource cannot be empty")
	}
	if operatorConfigName == "" {
		panic("observedConfigTarget: operatorConfigName cannot be empty")
	}
	if len(servingInfoPath) == 0 {
		panic("observedConfigTarget: servingInfoPath cannot be empty")
	}
	for i, segment := range servingInfoPath {
		if segment == "" {
			panic(fmt.Sprintf("observedConfigTarget: servingInfoPath[%d] cannot be empty", i))
		}
	}

	return observedConfigTarget{
		namespace:                  namespace,
		operatorConfigGVR:          operatorConfigGVR,
		operatorConfigName:         operatorConfigName,
		servingInfoPath:            servingInfoPath,
		managementClusterComponent: managementClusterComponent,
	}
}
