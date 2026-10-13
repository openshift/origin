package helm

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	helmv1beta1 "github.com/openshift/api/helm/v1beta1"
)

const (
	GroupName = "helm.openshift.io"
)

var (
	schemeBuilder = runtime.NewSchemeBuilder(helmv1beta1.Install)
	// Install is a function which adds every version of this group to a scheme
	Install = schemeBuilder.AddToScheme
)

func Resource(resource string) schema.GroupResource {
	return schema.GroupResource{Group: GroupName, Resource: resource}
}

func Kind(kind string) schema.GroupKind {
	return schema.GroupKind{Group: GroupName, Kind: kind}
}
