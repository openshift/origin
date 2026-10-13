package openshiftcontrolplane

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	openshiftcontrolplanev1 "github.com/openshift/api/openshiftcontrolplane/v1"
)

const (
	GroupName = "openshiftcontrolplane.config.openshift.io"
)

var (
	schemeBuilder = runtime.NewSchemeBuilder(openshiftcontrolplanev1.Install)
	// Install is a function which adds every version of this group to a scheme
	Install = schemeBuilder.AddToScheme
)

func Resource(resource string) schema.GroupResource {
	return schema.GroupResource{Group: GroupName, Resource: resource}
}

func Kind(kind string) schema.GroupKind {
	return schema.GroupKind{Group: GroupName, Kind: kind}
}
