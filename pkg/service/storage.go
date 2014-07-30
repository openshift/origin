package service

import (
	"fmt"

	baseapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/openshift/origin/pkg/api"
)

type ServiceRegistryStorage struct {
	registry ServiceRegistry
}

func NewRESTStorage(registry ServiceRegistry) apiserver.RESTStorage {
	return &ServiceRegistryStorage{registry}
}

func (s *ServiceRegistryStorage) New() interface{} {
	return &api.Service{}
}

func (s *ServiceRegistryStorage) Get(id string) (interface{}, error) {
	service, err := s.registry.GetService(id)
	if err != nil {
		return service, err
	}
	if service == nil {
		return service, nil
	}
	return service, err
}

func (s *ServiceRegistryStorage) List(selector labels.Selector) (interface{}, error) {
	var result api.ServiceList
	services, err := s.registry.ListServices(selector)
	if err == nil {
		result.Items = services
	}
	return result, err
}

func (s *ServiceRegistryStorage) Delete(id string) (<-chan interface{}, error) {
	return apiserver.MakeAsync(func() (interface{}, error) {
		return baseapi.Status{Status: baseapi.StatusSuccess}, s.registry.DeleteService(id)
	}), nil
}

func (s *ServiceRegistryStorage) Create(obj interface{}) (<-chan interface{}, error) {
	service := obj.(api.Service)
	if len(service.ID) == 0 {
		return nil, fmt.Errorf("id is unspecified: %#v", service)
	}

	return apiserver.MakeAsync(func() (interface{}, error) {
		if err := s.registry.CreateService(service); err != nil {
			return nil, err
		}
		return s.Get(service.ID)
	}), nil
}

func (s *ServiceRegistryStorage) Update(obj interface{}) (<-chan interface{}, error) {
	service := obj.(api.Service)
	if len(service.ID) == 0 {
		return nil, fmt.Errorf("id is unspecified: %#v", service)
	}

	return apiserver.MakeAsync(func() (interface{}, error) {
		err := s.registry.UpdateService(service)
		if err != nil {
			return nil, err
		}
		return s.Get(service.ID)
	}), nil
}
