package api

import (
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/runtime"

	_ "github.com/openshift/origin/pkg/authorization/api"
	_ "github.com/openshift/origin/pkg/build/api"
	_ "github.com/openshift/origin/pkg/deploy/api"
	_ "github.com/openshift/origin/pkg/image/api"
	_ "github.com/openshift/origin/pkg/oauth/api"
	_ "github.com/openshift/origin/pkg/project/api"
	_ "github.com/openshift/origin/pkg/route/api"
	_ "github.com/openshift/origin/pkg/sdn/api"
	_ "github.com/openshift/origin/pkg/template/api"
	_ "github.com/openshift/origin/pkg/user/api"
)

// SchemeGroupVersion is group version used to register these objects
var SchemeGroupVersion = unversioned.GroupVersion{Group: "", Version: ""}

// Codec is the identity codec for this package - it can only convert itself
// to itself.
var Codec = runtime.CodecFor(api.Scheme, SchemeGroupVersion.String())

// Kind takes an unqualified kind and returns back a Group qualified GroupKind
func Kind(kind string) unversioned.GroupKind {
	return SchemeGroupVersion.WithKind(kind).GroupKind()
}

// Resource takes an unqualified resource and returns back a Group qualified GroupResource
func Resource(resource string) unversioned.GroupResource {
	return SchemeGroupVersion.WithResource(resource).GroupResource()
}

func init() {
	api.Scheme.AddKnownTypes(SchemeGroupVersion)
}
