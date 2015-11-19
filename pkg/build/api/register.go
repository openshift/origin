package api

import (
	"k8s.io/kubernetes/pkg/api"
)

func init() {
	api.Scheme.AddKnownTypes("",
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
