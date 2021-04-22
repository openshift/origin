package apiserver

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	v1 "github.com/openshift/api/apiserver/v1"
)

var (
	schemeBuilder = runtime.NewSchemeBuilder(v1.Install)
	// Install is a function which adds every version of this group to a scheme
	Install = schemeBuilder.AddToScheme
)

func Resource(resource string) schema.GroupResource {
	return schema.GroupResource{Group: "apiserver.openshift.io", Resource: resource}
}

func Kind(kind string) schema.GroupKind {
	return schema.GroupKind{Group: "apiserver.openshift.io", Kind: kind}
}
