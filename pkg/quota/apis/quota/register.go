package quota

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kubernetes/pkg/apis/core"
)

const (
	GroupName = "quota.openshift.io"
)

var (
	schemeBuilder = runtime.NewSchemeBuilder(
		addKnownTypes,
		core.AddToScheme,
	)
	Install = schemeBuilder.AddToScheme

	// DEPRECATED kept for generated code
	SchemeGroupVersion = schema.GroupVersion{Group: GroupName, Version: runtime.APIVersionInternal}
	// DEPRECATED kept for generated code
	AddToScheme = schemeBuilder.AddToScheme
)

// Resource kept for generated code
// DEPRECATED
func Resource(resource string) schema.GroupResource {
	return SchemeGroupVersion.WithResource(resource).GroupResource()
}

// Adds the list of known types to api.Scheme.
func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemeGroupVersion,
		&ClusterResourceQuota{},
		&ClusterResourceQuotaList{},
		&AppliedClusterResourceQuota{},
		&AppliedClusterResourceQuotaList{},
	)
	return nil
}
