/*
Copyright 2016 The Kubernetes Authors.

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
	"github.com/kubernetes-incubator/service-catalog/pkg/api"
	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog"
	servicecatalogv1beta1 "github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/v1beta1"
	"github.com/kubernetes-incubator/service-catalog/pkg/registry/servicecatalog/binding"
	"github.com/kubernetes-incubator/service-catalog/pkg/registry/servicecatalog/broker"
	"github.com/kubernetes-incubator/service-catalog/pkg/registry/servicecatalog/instance"
	"github.com/kubernetes-incubator/service-catalog/pkg/registry/servicecatalog/server"
	"github.com/kubernetes-incubator/service-catalog/pkg/registry/servicecatalog/serviceclass"
	"github.com/kubernetes-incubator/service-catalog/pkg/registry/servicecatalog/serviceplan"
	"github.com/kubernetes-incubator/service-catalog/pkg/storage/etcd"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"
	serverstorage "k8s.io/apiserver/pkg/server/storage"
	"k8s.io/apiserver/pkg/storage"
	restclient "k8s.io/client-go/rest"
)

// StorageProvider provides a factory method to create a new APIGroupInfo for
// the servicecatalog API group. It implements (./pkg/apiserver).RESTStorageProvider
type StorageProvider struct {
	DefaultNamespace string
	StorageType      server.StorageType
	RESTClient       restclient.Interface
}

// NewRESTStorage is a factory method to make a new APIGroupInfo for the
// servicecatalog API group.
func (p StorageProvider) NewRESTStorage(
	apiResourceConfigSource serverstorage.APIResourceConfigSource,
	restOptionsGetter generic.RESTOptionsGetter,
) (*genericapiserver.APIGroupInfo, error) {

	storage, err := p.v1beta1Storage(apiResourceConfigSource, restOptionsGetter)
	if err != nil {
		return nil, err
	}

	apiGroupInfo := genericapiserver.NewDefaultAPIGroupInfo(servicecatalog.GroupName, api.Registry, api.Scheme, api.ParameterCodec, api.Codecs)
	apiGroupInfo.GroupMeta.GroupVersion = servicecatalogv1beta1.SchemeGroupVersion

	apiGroupInfo.VersionedResourcesStorageMap = map[string]map[string]rest.Storage{
		servicecatalogv1beta1.SchemeGroupVersion.Version: storage,
	}

	return &apiGroupInfo, nil
}

func (p StorageProvider) v1beta1Storage(
	apiResourceConfigSource serverstorage.APIResourceConfigSource,
	restOptionsGetter generic.RESTOptionsGetter,
) (map[string]rest.Storage, error) {
	brokerRESTOptions, err := restOptionsGetter.GetRESTOptions(servicecatalog.Resource("clusterservicebrokers"))
	if err != nil {
		return nil, err
	}
	brokerOpts := server.NewOptions(
		etcd.Options{
			RESTOptions:   brokerRESTOptions,
			Capacity:      1000,
			ObjectType:    broker.EmptyObject(),
			ScopeStrategy: broker.NewScopeStrategy(),
			NewListFunc:   broker.NewList,
			GetAttrsFunc:  broker.GetAttrs,
			Trigger:       storage.NoTriggerPublisher,
		},
		p.StorageType,
	)

	serviceClassRESTOptions, err := restOptionsGetter.GetRESTOptions(servicecatalog.Resource("clusterserviceclasses"))
	if err != nil {
		return nil, err
	}
	serviceClassOpts := server.NewOptions(
		etcd.Options{
			RESTOptions:   serviceClassRESTOptions,
			Capacity:      1000,
			ObjectType:    serviceclass.EmptyObject(),
			ScopeStrategy: serviceclass.NewScopeStrategy(),
			NewListFunc:   serviceclass.NewList,
			GetAttrsFunc:  serviceclass.GetAttrs,
			Trigger:       storage.NoTriggerPublisher,
		},
		p.StorageType,
	)

	servicePlanRESTOptions, err := restOptionsGetter.GetRESTOptions(servicecatalog.Resource("clusterserviceplans"))
	if err != nil {
		return nil, err
	}
	servicePlanOpts := server.NewOptions(
		etcd.Options{
			RESTOptions:   servicePlanRESTOptions,
			Capacity:      1000,
			ObjectType:    serviceplan.EmptyObject(),
			ScopeStrategy: serviceplan.NewScopeStrategy(),
			NewListFunc:   serviceplan.NewList,
			GetAttrsFunc:  serviceplan.GetAttrs,
			Trigger:       storage.NoTriggerPublisher,
		},
		p.StorageType,
	)

	instanceClassRESTOptions, err := restOptionsGetter.GetRESTOptions(servicecatalog.Resource("serviceinstances"))
	if err != nil {
		return nil, err
	}
	instanceOpts := server.NewOptions(
		etcd.Options{
			RESTOptions:   instanceClassRESTOptions,
			Capacity:      1000,
			ObjectType:    instance.EmptyObject(),
			ScopeStrategy: instance.NewScopeStrategy(),
			NewListFunc:   instance.NewList,
			GetAttrsFunc:  instance.GetAttrs,
			Trigger:       storage.NoTriggerPublisher,
		},
		p.StorageType,
	)

	bindingClassRESTOptions, err := restOptionsGetter.GetRESTOptions(servicecatalog.Resource("servicebindings"))
	if err != nil {
		return nil, err
	}
	bindingsOpts := server.NewOptions(
		etcd.Options{
			RESTOptions:   bindingClassRESTOptions,
			Capacity:      1000,
			ObjectType:    binding.EmptyObject(),
			ScopeStrategy: binding.NewScopeStrategy(),
			NewListFunc:   binding.NewList,
			GetAttrsFunc:  binding.GetAttrs,
			Trigger:       storage.NoTriggerPublisher,
		},
		p.StorageType,
	)

	brokerStorage, brokerStatusStorage := broker.NewStorage(*brokerOpts)
	serviceClassStorage, serviceClassStatusStorage := serviceclass.NewStorage(*serviceClassOpts)
	servicePlanStorage, servicePlanStatusStorage := serviceplan.NewStorage(*servicePlanOpts)
	instanceStorage, instanceStatusStorage, instanceReferencesStorage := instance.NewStorage(*instanceOpts)
	bindingStorage, bindingStatusStorage, err := binding.NewStorage(*bindingsOpts)
	if err != nil {
		return nil, err
	}

	return map[string]rest.Storage{
		"clusterservicebrokers":        brokerStorage,
		"clusterservicebrokers/status": brokerStatusStorage,
		"clusterserviceclasses":        serviceClassStorage,
		"clusterserviceclasses/status": serviceClassStatusStorage,
		"clusterserviceplans":          servicePlanStorage,
		"clusterserviceplans/status":   servicePlanStatusStorage,
		"serviceinstances":             instanceStorage,
		"serviceinstances/status":      instanceStatusStorage,
		"serviceinstances/reference":   instanceReferencesStorage,
		"servicebindings":              bindingStorage,
		"servicebindings/status":       bindingStatusStorage,
	}, nil
}

// GroupName returns the API group name.
func (p StorageProvider) GroupName() string {
	return servicecatalog.GroupName
}
