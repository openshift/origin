package v1beta3

import (
	"k8s.io/kubernetes/pkg/api"

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
func (*ImageStreamDeletion) IsAnAPIObject()     {}
func (*ImageStreamDeletionList) IsAnAPIObject() {}
