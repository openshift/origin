package v1beta1

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"

	_ "github.com/openshift/origin/pkg/image/api/docker10"
	_ "github.com/openshift/origin/pkg/image/api/dockerpre012"
)

func init() {
	api.Scheme.AddKnownTypes("v1beta1",
		&Image{},
		&ImageList{},
		&ImageRepository{},
		&ImageRepositoryList{},
		&ImageRepositoryMapping{},
		&ImageRepositoryTag{},
		&ImageStream{},
		&ImageStreamList{},
		&ImageStreamMapping{},
		&ImageStreamTag{},
		&ImageStreamImage{},
	)
}

func (*Image) IsAnAPIObject()                  {}
func (*ImageList) IsAnAPIObject()              {}
func (*ImageRepository) IsAnAPIObject()        {}
func (*ImageRepositoryList) IsAnAPIObject()    {}
func (*ImageRepositoryMapping) IsAnAPIObject() {}
func (*ImageStream) IsAnAPIObject()            {}
func (*ImageStreamList) IsAnAPIObject()        {}
func (*ImageStreamMapping) IsAnAPIObject()     {}
func (*ImageStreamTag) IsAnAPIObject()         {}
