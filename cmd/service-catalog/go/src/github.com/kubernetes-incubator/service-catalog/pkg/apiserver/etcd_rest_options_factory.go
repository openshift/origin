/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package apiserver

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/server/storage"
)

// BABYNETES: had to be lifted from pkg/master/master.go

// restOptionsFactory is an object that provides a factory method for getting
// the REST options for a particular GroupResource.
type etcdRESTOptionsFactory struct {
	deleteCollectionWorkers int
	enableGarbageCollection bool
	storageFactory          storage.StorageFactory
	storageDecorator        generic.StorageDecorator
}

// NewFor returns the RESTOptions for a particular GroupResource.
func (f etcdRESTOptionsFactory) GetRESTOptions(resource schema.GroupResource) (generic.RESTOptions, error) {
	storageConfig, err := f.storageFactory.NewConfig(resource)
	if err != nil {
		return generic.RESTOptions{}, err
	}

	return generic.RESTOptions{
		StorageConfig:           storageConfig,
		Decorator:               f.storageDecorator,
		DeleteCollectionWorkers: f.deleteCollectionWorkers,
		EnableGarbageCollection: f.enableGarbageCollection,
		ResourcePrefix:          resource.Group + "/" + resource.Resource,
	}, nil
}
