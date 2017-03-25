package v1

import (
	"k8s.io/kubernetes/pkg/api/unversioned"
	kapiv1 "k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/watch/versioned"
)

const (
	GroupName       = "template.openshift.io"
	LegacyGroupName = ""
)

// SchemeGroupVersion is group version used to register these objects
var (
	SchemeGroupVersion       = unversioned.GroupVersion{Group: GroupName, Version: "v1"}
	LegacySchemeGroupVersion = unversioned.GroupVersion{Group: LegacyGroupName, Version: "v1"}

	LegacySchemeBuilder    = runtime.NewSchemeBuilder(addLegacyKnownTypes, addConversionFuncs)
	AddToSchemeInCoreGroup = LegacySchemeBuilder.AddToScheme

	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes, addConversionFuncs)
	AddToScheme   = SchemeBuilder.AddToScheme
)

// Adds the list of known types to api.Scheme.
func addKnownTypes(scheme *runtime.Scheme) error {
	types := []runtime.Object{
		&Template{},
		&TemplateList{},
		&TemplateInstance{},
		&TemplateInstanceList{},
		&BrokerTemplateInstance{},
		&BrokerTemplateInstanceList{},
	}
	scheme.AddKnownTypes(SchemeGroupVersion,
		append(types,
			&unversioned.Status{}, // TODO: revisit in 1.6 when Status is actually registered as unversioned
			&kapiv1.ListOptions{},
			&kapiv1.DeleteOptions{},
			&kapiv1.ExportOptions{},
			&kapiv1.List{},
		)...,
	)
	versioned.AddToGroupVersion(scheme, SchemeGroupVersion)
	return nil
}

func addLegacyKnownTypes(scheme *runtime.Scheme) error {
	types := []runtime.Object{
		&Template{},
		&TemplateList{},
	}
	scheme.AddKnownTypes(LegacySchemeGroupVersion, types...)
	scheme.AddKnownTypeWithName(LegacySchemeGroupVersion.WithKind("TemplateConfig"), &Template{})
	scheme.AddKnownTypeWithName(LegacySchemeGroupVersion.WithKind("ProcessedTemplate"), &Template{})
	return nil
}
