package api

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kubernetes/pkg/watch/versioned"
)

const (
	GroupName       = "requestlimit.project.openshift.io"
	LegacyGroupName = ""
)

var (
	SchemeGroupVersion       = schema.GroupVersion{Group: GroupName, Version: runtime.APIVersionInternal}
	LegacySchemeGroupVersion = schema.GroupVersion{Group: LegacyGroupName, Version: runtime.APIVersionInternal}

	LegacySchemeBuilder    = runtime.NewSchemeBuilder(addLegacyKnownTypes)
	AddToSchemeInCoreGroup = LegacySchemeBuilder.AddToScheme

	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
	AddToScheme   = SchemeBuilder.AddToScheme
)

// Adds the list of known types to api.Scheme.
func addLegacyKnownTypes(scheme *runtime.Scheme) error {
	types := []runtime.Object{
		&ProjectRequestLimitConfig{},
	}
	scheme.AddKnownTypes(LegacySchemeGroupVersion, types...)
	return nil
}

// Adds the list of known types to api.Scheme.
func addKnownTypes(scheme *runtime.Scheme) error {
	types := []runtime.Object{
		&ProjectRequestLimitConfig{},
	}
	scheme.AddKnownTypes(SchemeGroupVersion,
		append(types,
			&metav1.Status{}, // TODO: revisit in 1.6 when Status is actually registered as unversioned
			&metainternal.ListOptions{},
			&metainternal.DeleteOptions{},
			&metainternal.ExportOptions{},
		)...,
	)
	versioned.AddToGroupVersion(scheme, SchemeGroupVersion)
	return nil
}
