package v1

import (
	"github.com/openshift/api/build/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const GroupName = "build.openshift.io"
const LegacyGroupName = ""

var (
	SchemeGroupVersion       = schema.GroupVersion{Group: GroupName, Version: "v1"}
	LegacySchemeGroupVersion = schema.GroupVersion{Group: LegacyGroupName, Version: "v1"}

	LegacySchemeBuilder    = runtime.NewSchemeBuilder(v1.LegacySchemeBuilder.AddToScheme, addConversionFuncs, addLegacyFieldSelectorKeyConversions, RegisterDefaults, RegisterConversions)
	AddToSchemeInCoreGroup = LegacySchemeBuilder.AddToScheme

	SchemeBuilder = runtime.NewSchemeBuilder(v1.SchemeBuilder.AddToScheme, addConversionFuncs, addFieldSelectorKeyConversions, RegisterDefaults)
	AddToScheme   = SchemeBuilder.AddToScheme

	localSchemeBuilder = &SchemeBuilder
)

func Resource(resource string) schema.GroupResource {
	return SchemeGroupVersion.WithResource(resource).GroupResource()
}

// LegacyResource takes an unqualified resource and returns back a Group qualified GroupResource
func LegacyResource(resource string) schema.GroupResource {
	return LegacySchemeGroupVersion.WithResource(resource).GroupResource()
}

// IsResourceOrLegacy checks if the provided GroupResources matches with the given
// resource by looking up the API group and also the legacy API.
func IsResourceOrLegacy(resource string, gr schema.GroupResource) bool {
	return gr == Resource(resource) || gr == LegacyResource(resource)
}
