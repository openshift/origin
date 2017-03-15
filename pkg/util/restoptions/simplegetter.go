package restoptions

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/storage/storagebackend"
	genericrest "k8s.io/kubernetes/pkg/registry/generic"
)

type simpleGetter struct {
	storage *storagebackend.Config
}

func NewSimpleGetter(storage *storagebackend.Config) Getter {
	return &simpleGetter{storage: storage}
}

func (s *simpleGetter) GetRESTOptions(resource schema.GroupResource) (genericrest.RESTOptions, error) {
	return genericrest.RESTOptions{
		StorageConfig:           s.storage,
		Decorator:               genericrest.UndecoratedStorage,
		DeleteCollectionWorkers: 1,
		ResourcePrefix:          resource.Resource,
	}, nil
}
