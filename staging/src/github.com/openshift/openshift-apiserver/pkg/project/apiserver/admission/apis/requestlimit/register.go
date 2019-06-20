package requestlimit

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	GroupName = "project.openshift.io"
)

var (
	GroupVersion = schema.GroupVersion{Group: GroupName, Version: runtime.APIVersionInternal}

	schemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
	Install       = schemeBuilder.AddToScheme
)

// Adds the list of known types to api.Scheme.
func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(GroupVersion,
		&ProjectRequestLimitConfig{},
	)
	return nil
}
