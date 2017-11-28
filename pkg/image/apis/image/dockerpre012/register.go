package dockerpre012

import (
	"github.com/openshift/api/image/dockerpre012"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	GroupName       = "image.openshift.io"
	LegacyGroupName = ""
)

var (
	SchemeGroupVersion       = schema.GroupVersion{Group: GroupName, Version: "pre012"}
	LegacySchemeGroupVersion = schema.GroupVersion{Group: LegacyGroupName, Version: "pre012"}

	SchemeBuilder = runtime.NewSchemeBuilder(addConversionFuncs, dockerpre012.RegisterDeepCopies)
	AddToScheme   = SchemeBuilder.AddToScheme

	LegacySchemeBuilder    = runtime.NewSchemeBuilder(addConversionFuncs)
	AddToSchemeInCoreGroup = LegacySchemeBuilder.AddToScheme
)
