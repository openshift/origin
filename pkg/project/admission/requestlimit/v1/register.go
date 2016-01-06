package v1

import (
	"k8s.io/kubernetes/pkg/api/unversioned"

	"github.com/openshift/origin/pkg/cmd/server/api"
)

// SchemeGroupVersion is group version used to register these objects
var SchemeGroupVersion = unversioned.GroupVersion{Group: "", Version: "v1"}

func init() {
	api.Scheme.AddKnownTypes(SchemeGroupVersion,
		&ProjectRequestLimitConfig{},
	)
}

func (*ProjectRequestLimitConfig) IsAnAPIObject() {}
