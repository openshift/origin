package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kapiv1 "k8s.io/kubernetes/pkg/api/v1"
)

const GroupName = "build.openshift.io"
const LegacyGroupName = ""

var (
	SchemeGroupVersion       = schema.GroupVersion{Group: GroupName, Version: "v1"}
	LegacySchemeGroupVersion = schema.GroupVersion{Group: LegacyGroupName, Version: "v1"}

	LegacySchemeBuilder    = runtime.NewSchemeBuilder(addLegacyKnownTypes, addConversionFuncs, addLegacyFieldSelectorKeyConversions, RegisterDefaults)
	AddToSchemeInCoreGroup = LegacySchemeBuilder.AddToScheme

	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes, addConversionFuncs, addFieldSelectorKeyConversions, RegisterDefaults)
	AddToScheme   = SchemeBuilder.AddToScheme
)

func Resource(resource string) schema.GroupResource {
	return SchemeGroupVersion.WithResource(resource).GroupResource()
}

// addKnownTypes adds types to API group
func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemeGroupVersion,
		&Build{},
		&BuildList{},
		&BuildConfig{},
		&BuildConfigList{},
		&BuildLog{},
		&BuildRequest{},
		&BuildLogOptions{},
		&BinaryBuildRequestOptions{},
		// This is needed for webhooks
		&kapiv1.PodProxyOptions{},
	)
	metav1.AddToGroupVersion(scheme, SchemeGroupVersion)
	return nil
}

// addLegacyKnownTypes adds types to legacy API group
// DEPRECATED: This will be deprecated and should not be modified.
func addLegacyKnownTypes(scheme *runtime.Scheme) error {
	types := []runtime.Object{
		&Build{},
		&BuildList{},
		&BuildConfig{},
		&BuildConfigList{},
		&BuildLog{},
		&BuildRequest{},
		&BuildLogOptions{},
		&BinaryBuildRequestOptions{},
	}
	scheme.AddKnownTypes(LegacySchemeGroupVersion, types...)
	return nil
}
