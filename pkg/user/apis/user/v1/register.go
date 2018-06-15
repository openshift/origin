package v1

import (
	"github.com/openshift/api/user/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	GroupName       = "user.openshift.io"
	LegacyGroupName = ""
)

// SchemeGroupVersion is group version used to register these objects
var (
	SchemeGroupVersion       = schema.GroupVersion{Group: GroupName, Version: "v1"}
	LegacySchemeGroupVersion = schema.GroupVersion{Group: LegacyGroupName, Version: "v1"}

	LegacySchemeBuilder    = runtime.NewSchemeBuilder(v1.LegacySchemeBuilder.AddToScheme, addConversionFuncs, addLegacyFieldSelectorKeyConversions, RegisterDefaults, RegisterConversions)
	AddToSchemeInCoreGroup = LegacySchemeBuilder.AddToScheme

	SchemeBuilder = runtime.NewSchemeBuilder(v1.SchemeBuilder.AddToScheme, addConversionFuncs, addFieldSelectorKeyConversions)
	AddToScheme   = SchemeBuilder.AddToScheme

	localSchemeBuilder = &SchemeBuilder
)

func Resource(resource string) schema.GroupResource {
	return SchemeGroupVersion.WithResource(resource).GroupResource()
}
