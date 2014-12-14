package v1beta1

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
)

func init() {
	api.Scheme.AddKnownTypes("v1beta1",
		&Build{},
		&BuildList{},
		&BuildConfig{},
		&BuildConfigList{},
	)
	api.Scheme.AddKnownTypeWithName("v1beta1", "BuildLog", &Build{})
}

func (*Build) IsAnAPIObject()           {}
func (*BuildList) IsAnAPIObject()       {}
func (*BuildConfig) IsAnAPIObject()     {}
func (*BuildConfigList) IsAnAPIObject() {}
