package api

import (
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
)

const GroupName = ""

// SchemeGroupVersion is group version used to register these objects
var SchemeGroupVersion = unversioned.GroupVersion{Group: GroupName, Version: ""}

// Kind takes an unqualified kind and returns back a Group qualified GroupKind
func Kind(kind string) unversioned.GroupKind {
	return SchemeGroupVersion.WithKind(kind).GroupKind()
}

// Resource takes an unqualified resource and returns back a Group qualified GroupResource
func Resource(resource string) unversioned.GroupResource {
	return SchemeGroupVersion.WithResource(resource).GroupResource()
}

func init() {
	api.Scheme.AddKnownTypes(SchemeGroupVersion,
		&Image{},
		&ImageList{},
		&DockerImage{},
		&ImageStream{},
		&ImageStreamList{},
		&ImageStreamMapping{},
		&ImageStreamTag{},
		&ImageStreamTagList{},
		&ImageStreamImage{},
		&ImageStreamImport{},
	)
}

func (*Image) IsAnAPIObject()              {}
func (*ImageList) IsAnAPIObject()          {}
func (*DockerImage) IsAnAPIObject()        {}
func (*ImageStream) IsAnAPIObject()        {}
func (*ImageStreamList) IsAnAPIObject()    {}
func (*ImageStreamMapping) IsAnAPIObject() {}
func (*ImageStreamTag) IsAnAPIObject()     {}
func (*ImageStreamTagList) IsAnAPIObject() {}
func (*ImageStreamImage) IsAnAPIObject()   {}
func (*ImageStreamImport) IsAnAPIObject()  {}
