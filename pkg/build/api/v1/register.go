package v1

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
)

func init() {
	api.Scheme.AddKnownTypes("v1",
		&Build{},
		&BuildList{},
		&BuildConfig{},
		&BuildConfigList{},
		&BuildLog{},
		&BuildRequest{},
		&BuildLogOptions{},
	)
}

func (*Build) IsAnAPIObject()           {}
func (*BuildList) IsAnAPIObject()       {}
func (*BuildConfig) IsAnAPIObject()     {}
func (*BuildConfigList) IsAnAPIObject() {}
func (*BuildLog) IsAnAPIObject()        {}
func (*BuildRequest) IsAnAPIObject()    {}
func (*BuildLogOptions) IsAnAPIObject() {}
