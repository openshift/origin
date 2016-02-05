package requestlimit

import (
	"k8s.io/kubernetes/pkg/api/unversioned"

	"github.com/openshift/origin/pkg/cmd/server/api"
	_ "github.com/openshift/origin/pkg/project/admission/requestlimit/latest"
)

// SchemeGroupVersion is group version used to register these objects
var SchemeGroupVersion = unversioned.GroupVersion{Group: "", Version: ""}

// Kind takes an unqualified kind and returns back a Group qualified GroupKind
func Kind(kind string) unversioned.GroupKind {
	return SchemeGroupVersion.WithKind(kind).GroupKind()
}

// Resource takes an unqualified resource and returns back a Group qualified GroupResource
func Resource(resource string) unversioned.GroupResource {
	return SchemeGroupVersion.WithResource(resource).GroupResource()
}

func init() {
	api.Scheme.AddKnownTypes(SchemeGroupVersion,
		&ProjectRequestLimitConfig{},
	)
}

func (*ProjectRequestLimitConfig) IsAnAPIObject() {}
