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

type tprRESTOptionsFactory struct {
	storageFactory storage.StorageFactory
}

func (t tprRESTOptionsFactory) GetRESTOptions(resource schema.GroupResource) (generic.RESTOptions, error) {
	storageConfig, err := t.storageFactory.NewConfig(resource)
	if err != nil {
		return generic.RESTOptions{}, err
	}
	// this function should create a RESTOptions that contains a Decorator function to create
	// a TPR based storage config. This should be done in a follow up to
	// https://github.com/kubernetes-incubator/service-catalog/pull/338. When this decorator
	// function is implemented, the 'NewStorage' functions in
	// ./pkg/registry/servicecatalog/{binding,broker,instance,serviceclass} no longer will need
	// to have the switching logic to choose between TPR and etcd
	return generic.RESTOptions{
		StorageConfig: storageConfig,
	}, nil
}
