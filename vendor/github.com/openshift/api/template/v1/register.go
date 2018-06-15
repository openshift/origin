package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	GroupName       = "template.openshift.io"
	LegacyGroupName = ""
)

// SchemeGroupVersion is group version used to register these objects
var (
	SchemeGroupVersion       = schema.GroupVersion{Group: GroupName, Version: "v1"}
	LegacySchemeGroupVersion = schema.GroupVersion{Group: LegacyGroupName, Version: "v1"}

	LegacySchemeBuilder    = runtime.NewSchemeBuilder(addLegacyKnownTypes)
	AddToSchemeInCoreGroup = LegacySchemeBuilder.AddToScheme

	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
	AddToScheme   = SchemeBuilder.AddToScheme
)

func Resource(resource string) schema.GroupResource {
	return SchemeGroupVersion.WithResource(resource).GroupResource()
}

// Adds the list of known types to api.Scheme.
func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemeGroupVersion,
		&Template{},
		&TemplateList{},
		&TemplateInstance{},
		&TemplateInstanceList{},
		&BrokerTemplateInstance{},
		&BrokerTemplateInstanceList{},
		&corev1.List{},
	)
	metav1.AddToGroupVersion(scheme, SchemeGroupVersion)
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
