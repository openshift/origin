package restoptions

import (
	"k8s.io/kubernetes/pkg/api/unversioned"
	genericrest "k8s.io/kubernetes/pkg/registry/generic"
	"k8s.io/kubernetes/pkg/storage"
)

type simpleGetter struct {
	storage storage.Interface
}

func NewSimpleGetter(storage storage.Interface) Getter {
	return &simpleGetter{storage: storage}
}

func (s *simpleGetter) GetRESTOptions(resource unversioned.GroupResource) (genericrest.RESTOptions, error) {
	return genericrest.RESTOptions{
		Storage:                 s.storage,
		Decorator:               genericrest.UndecoratedStorage,
		DeleteCollectionWorkers: 1,
	}, nil
}
