package restoptions

import (
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/registry/generic"
)

type Getter interface {
	GetRESTOptions(resource unversioned.GroupResource) (generic.RESTOptions, error)
}

type UpstreamGetterFunc func(resource unversioned.GroupResource) generic.RESTOptions

func (f UpstreamGetterFunc) GetRESTOptions(resource unversioned.GroupResource) (generic.RESTOptions, error) {
	options := f(resource)
	return options, nil
}
