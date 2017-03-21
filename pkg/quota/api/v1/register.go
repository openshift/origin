package v1

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/watch/versioned"
)

const (
	GroupName       = "quota.openshift.io"
	LegacyGroupName = ""
)

// SchemeGroupVersion is group version used to register these objects
var (
	SchemeGroupVersion       = unversioned.GroupVersion{Group: GroupName, Version: "v1"}
	LegacySchemeGroupVersion = unversioned.GroupVersion{Group: LegacyGroupName, Version: "v1"}

	LegacySchemeBuilder    = runtime.NewSchemeBuilder(addLegacyKnownTypes)
	AddToSchemeInCoreGroup = LegacySchemeBuilder.AddToScheme

	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
	AddToScheme   = SchemeBuilder.AddToScheme
)

// Kind takes an unqualified kind and returns back a Group qualified GroupKind
func Kind(kind string) unversioned.GroupKind {
	return SchemeGroupVersion.WithKind(kind).GroupKind()
}

// Resource takes an unqualified resource and returns back a Group qualified GroupResource
func Resource(resource string) unversioned.GroupResource {
	return SchemeGroupVersion.WithResource(resource).GroupResource()
}

// Adds the list of known types to api.Scheme.
func addKnownTypes(scheme *runtime.Scheme) error {
	types := []runtime.Object{
		&ClusterResourceQuota{},
		&ClusterResourceQuotaList{},
		&AppliedClusterResourceQuota{},
		&AppliedClusterResourceQuotaList{},
	}
	scheme.AddKnownTypes(SchemeGroupVersion,
		append(types,
			&unversioned.Status{}, // TODO: revisit in 1.6 when Status is actually registered as unversioned
			&kapi.ListOptions{},
			&kapi.DeleteOptions{},
			&kapi.ExportOptions{},
		)...,
	)
	versioned.AddToGroupVersion(scheme, SchemeGroupVersion)
	return nil
}

func addLegacyKnownTypes(scheme *runtime.Scheme) error {
	types := []runtime.Object{
		&ClusterResourceQuota{},
		&ClusterResourceQuotaList{},
		&AppliedClusterResourceQuota{},
		&AppliedClusterResourceQuotaList{},
	}
	scheme.AddKnownTypes(LegacySchemeGroupVersion, types...)
	return nil
}
