package restoptions

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/storage/storagebackend"
)

type simpleGetter struct {
	storage *storagebackend.Config
}

func NewSimpleGetter(storage *storagebackend.Config) Getter {
	return &simpleGetter{storage: storage}
}

func (s *simpleGetter) GetRESTOptions(resource schema.GroupResource) (generic.RESTOptions, error) {
	return generic.RESTOptions{
		StorageConfig:           s.storage,
		Decorator:               generic.UndecoratedStorage,
		DeleteCollectionWorkers: 1,
		ResourcePrefix:          resource.Resource,
	}, nil
}
