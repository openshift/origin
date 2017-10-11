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

package rest

import (
	"testing"

	"github.com/kubernetes-incubator/service-catalog/pkg/registry/servicecatalog/server"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/registry/generic"
	serverstorage "k8s.io/apiserver/pkg/server/storage"
	"k8s.io/apiserver/pkg/storage"
	"k8s.io/apiserver/pkg/storage/storagebackend"
	"k8s.io/apiserver/pkg/storage/storagebackend/factory"
)

type GetRESTOptionsHelper struct {
	retStorageInterface storage.Interface
	retDestroyFunc      func()
}

func (g GetRESTOptionsHelper) GetRESTOptions(resource schema.GroupResource) (generic.RESTOptions, error) {
	return generic.RESTOptions{
		StorageConfig: &storagebackend.Config{},
		Decorator: generic.StorageDecorator(func(
			copier runtime.ObjectCopier,
			config *storagebackend.Config,
			objectType runtime.Object,
			resourcePrefix string,
			keyFunc func(obj runtime.Object) (string, error),
			newListFunc func() runtime.Object,
			getAttrsFunc storage.AttrFunc,
			trigger storage.TriggerPublisherFunc,
		) (storage.Interface, factory.DestroyFunc) {
			return g.retStorageInterface, g.retDestroyFunc
		})}, nil
}

func testRESTOptionsGetter(
	retStorageInterface storage.Interface,
	retDestroyFunc func(),
) generic.RESTOptionsGetter {
	return GetRESTOptionsHelper{retStorageInterface, retDestroyFunc}
}

func TestV1Alpha1Storage(t *testing.T) {
	provider := StorageProvider{
		DefaultNamespace: "test-default",
		StorageType:      server.StorageTypeEtcd,
		RESTClient:       nil,
	}
	configSource := serverstorage.NewResourceConfig()
	roGetter := testRESTOptionsGetter(nil, func() {})
	storageMap, err := provider.v1beta1Storage(configSource, roGetter)
	if err != nil {
		t.Fatalf("error getting v1beta1 storage (%s)", err)
	}
	_, brokerStorageExists := storageMap["clusterservicebrokers"]
	if !brokerStorageExists {
		t.Fatalf("no broker storage found")
	}
	// TODO: do stuff with broker storage
	_, brokerStatusStorageExists := storageMap["clusterservicebrokers/status"]
	if !brokerStatusStorageExists {
		t.Fatalf("no service broker status storage found")
	}
	// TODO: do stuff with broker status storage

	_, serviceClassStorageExists := storageMap["clusterserviceclasses"]
	if !serviceClassStorageExists {
		t.Fatalf("no service class storage found")
	}
	// TODO: do stuff with service class storage

	_, instanceStorageExists := storageMap["serviceinstances"]
	if !instanceStorageExists {
		t.Fatalf("no service instance storage found")
	}
	// TODO: do stuff with instance storage

	_, bindingStorageExists := storageMap["servicebindings"]
	if !bindingStorageExists {
		t.Fatalf("no service instance credential storage found")
	}
	// TODO: do stuff with binding storage

}
