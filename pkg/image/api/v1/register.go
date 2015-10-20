package v1

import (
	"k8s.io/kubernetes/pkg/api"

	_ "github.com/openshift/origin/pkg/image/api/docker10"
	_ "github.com/openshift/origin/pkg/image/api/dockerpre012"
)

func init() {
	api.Scheme.AddKnownTypes("v1",
		&Image{},
		&ImageList{},
		&ImageStream{},
		&ImageStreamList{},
		&ImageStreamMapping{},
		&ImageStreamTag{},
		&ImageStreamImage{},
		&ImageStreamImageList{},
		&ImageStreamDeletion{},
		&ImageStreamDeletionList{},
	)
}

func (*Image) IsAnAPIObject()                   {}
func (*ImageList) IsAnAPIObject()               {}
func (*ImageStream) IsAnAPIObject()             {}
func (*ImageStreamList) IsAnAPIObject()         {}
func (*ImageStreamMapping) IsAnAPIObject()      {}
func (*ImageStreamTag) IsAnAPIObject()          {}
func (*ImageStreamImage) IsAnAPIObject()        {}
func (*ImageStreamImageList) IsAnAPIObject()    {}
func (*ImageStreamDeletion) IsAnAPIObject()     {}
func (*ImageStreamDeletionList) IsAnAPIObject() {}
