package cloudnetwork

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	cloudnetworkv1 "github.com/openshift/api/cloudnetwork/v1"
)

const (
	GroupName = "cloud.network.openshift.io"
)

var (
	schemeBuilder = runtime.NewSchemeBuilder(cloudnetworkv1.Install)
	// Install is a function which adds every version of this group to a scheme
	Install = schemeBuilder.AddToScheme
)

func Resource(resource string) schema.GroupResource {
	return schema.GroupResource{Group: GroupName, Resource: resource}
}

func Kind(kind string) schema.GroupKind {
	return schema.GroupKind{Group: GroupName, Kind: kind}
}
