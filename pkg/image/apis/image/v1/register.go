package v1

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/openshift/api/image/docker10"
	"github.com/openshift/api/image/dockerpre012"
	"github.com/openshift/api/image/v1"
)

const (
	GroupName       = "image.openshift.io"
	LegacyGroupName = ""
)

var (
	SchemeGroupVersion       = schema.GroupVersion{Group: GroupName, Version: "v1"}
	LegacySchemeGroupVersion = schema.GroupVersion{Group: LegacyGroupName, Version: "v1"}

	LegacySchemeBuilder    = runtime.NewSchemeBuilder(v1.LegacySchemeBuilder.AddToScheme, addConversionFuncs, addLegacyFieldSelectorKeyConversions, RegisterDefaults, RegisterConversions, docker10.AddToSchemeInCoreGroup, dockerpre012.AddToSchemeInCoreGroup)
	AddToSchemeInCoreGroup = LegacySchemeBuilder.AddToScheme

	SchemeBuilder = runtime.NewSchemeBuilder(v1.SchemeBuilder.AddToScheme, addConversionFuncs, addFieldSelectorKeyConversions, RegisterDefaults, docker10.AddToScheme, dockerpre012.AddToScheme)
	AddToScheme   = SchemeBuilder.AddToScheme

	localSchemeBuilder = &SchemeBuilder
)

func Resource(resource string) schema.GroupResource {
	return SchemeGroupVersion.WithResource(resource).GroupResource()
}
