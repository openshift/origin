package v1

import (
	"k8s.io/kubernetes/pkg/api/unversioned"

	"github.com/openshift/origin/pkg/cmd/server/api"
)

// SchemeGroupVersion is group version used to register these objects
var SchemeGroupVersion = unversioned.GroupVersion{Group: "", Version: "v1"}

// Codec encodes internal objects to the v1 scheme
//var Codec = runtime.CodecFor(api.Scheme, SchemeGroupVersion.String())

func init() {
	api.Scheme.AddKnownTypes(SchemeGroupVersion,
		&DefaultConfig{},
	)
}

func (*DefaultConfig) IsAnAPIObject() {}
