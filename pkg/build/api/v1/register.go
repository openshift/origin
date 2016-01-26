package v1

import (
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/runtime"
)

// SchemeGroupVersion is group version used to register these objects
var SchemeGroupVersion = unversioned.GroupVersion{Group: "", Version: "v1"}

// Codec encodes internal objects to the v1 scheme
var Codec = runtime.CodecFor(api.Scheme, SchemeGroupVersion.String())

func init() {
	api.Scheme.AddKnownTypes(SchemeGroupVersion,
		&Build{},
		&BuildList{},
		&BuildConfig{},
		&BuildConfigList{},
		&BuildLog{},
		&BuildRequest{},
		&BuildLogOptions{},
		&BinaryBuildRequestOptions{},
	)
}

func (*Build) IsAnAPIObject()                     {}
func (*BuildList) IsAnAPIObject()                 {}
func (*BuildConfig) IsAnAPIObject()               {}
func (*BuildConfigList) IsAnAPIObject()           {}
func (*BuildLog) IsAnAPIObject()                  {}
func (*BuildRequest) IsAnAPIObject()              {}
func (*BuildLogOptions) IsAnAPIObject()           {}
func (*BinaryBuildRequestOptions) IsAnAPIObject() {}
