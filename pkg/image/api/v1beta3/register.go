package v1beta3

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"

	_ "github.com/openshift/origin/pkg/image/api/docker10"
	_ "github.com/openshift/origin/pkg/image/api/dockerpre012"
)

func init() {
	api.Scheme.AddKnownTypes("v1beta3",
		&Image{},
		&ImageList{},
		&ImageStream{},
		&ImageStreamList{},
		&ImageStreamMapping{},
		&ImageStreamTag{},
		&ImageStreamImage{},
	)
	// Legacy names are supported
	api.Scheme.AddKnownTypeWithName("v1beta3", "ImageRepository", &ImageStream{})
	api.Scheme.AddKnownTypeWithName("v1beta3", "ImageRepositoryList", &ImageStreamList{})
	api.Scheme.AddKnownTypeWithName("v1beta3", "ImageRepositoryMapping", &ImageStreamMapping{})
	api.Scheme.AddKnownTypeWithName("v1beta3", "ImageRepositoryTag", &ImageStreamTag{})
	api.Scheme.AddKnownTypeWithName("v1beta3", "ImageRepositoryImage", &ImageStreamImage{})
}

func (*Image) IsAnAPIObject()              {}
func (*ImageList) IsAnAPIObject()          {}
func (*ImageStream) IsAnAPIObject()        {}
func (*ImageStreamList) IsAnAPIObject()    {}
func (*ImageStreamMapping) IsAnAPIObject() {}
func (*ImageStreamTag) IsAnAPIObject()     {}
