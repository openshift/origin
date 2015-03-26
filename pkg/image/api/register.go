package api

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
)

func init() {
	api.Scheme.AddKnownTypes("",
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
		&DockerImage{},
	)
}

func (*Image) IsAnAPIObject()                  {}
func (*ImageList) IsAnAPIObject()              {}
func (*ImageRepository) IsAnAPIObject()        {}
func (*ImageRepositoryList) IsAnAPIObject()    {}
func (*ImageRepositoryMapping) IsAnAPIObject() {}
func (*DockerImage) IsAnAPIObject()            {}
func (*ImageStream) IsAnAPIObject()            {}
func (*ImageStreamList) IsAnAPIObject()        {}
func (*ImageStreamMapping) IsAnAPIObject()     {}
func (*ImageStreamTag) IsAnAPIObject()         {}
