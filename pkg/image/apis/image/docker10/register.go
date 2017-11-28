package docker10

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	GroupName       = "image.openshift.io"
	LegacyGroupName = ""
)

// SchemeGroupVersion is group version used to register these objects
var (
	SchemeGroupVersion       = schema.GroupVersion{Group: GroupName, Version: "1.0"}
	LegacySchemeGroupVersion = schema.GroupVersion{Group: LegacyGroupName, Version: "1.0"}

	SchemeBuilder       = runtime.NewSchemeBuilder()
	LegacySchemeBuilder = runtime.NewSchemeBuilder()

	AddToScheme            = SchemeBuilder.AddToScheme
	AddToSchemeInCoreGroup = LegacySchemeBuilder.AddToScheme
)
