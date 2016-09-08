package restoptions

import (
	"k8s.io/kubernetes/pkg/api/unversioned"
	genericrest "k8s.io/kubernetes/pkg/registry/generic"
	"k8s.io/kubernetes/pkg/storage/storagebackend"
)

type simpleGetter struct {
	storage *storagebackend.Config
}

func NewSimpleGetter(storage *storagebackend.Config) Getter {
	return &simpleGetter{storage: storage}
}

func (s *simpleGetter) GetRESTOptions(resource unversioned.GroupResource) (genericrest.RESTOptions, error) {
	return genericrest.RESTOptions{
		StorageConfig:           s.storage,
		Decorator:               genericrest.UndecoratedStorage,
		DeleteCollectionWorkers: 1,
		ResourcePrefix:          resource.Resource,
	}, nil
}
