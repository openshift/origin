package service

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/openshift/origin/pkg/api"
)

// Mainly used for testing.
type MemoryRegistry struct {
	serviceData map[string]api.Service
}

func MakeMemoryRegistry() *MemoryRegistry {
	return &MemoryRegistry{
		serviceData: map[string]api.Service{},
	}
}

func (registry *MemoryRegistry) ListServices(selector labels.Selector) ([]api.Service, error) {
	result := []api.Service{}
	for _, value := range registry.serviceData {
		if selector.Matches(labels.Set(value.Labels)) {
			result = append(result, value)
		}
	}
	return result, nil
}

func (registry *MemoryRegistry) GetService(serviceID string) (*api.Service, error) {
	service, found := registry.serviceData[serviceID]
	if found {
		return &service, nil
	} else {
		return nil, nil
	}
}

func (registry *MemoryRegistry) CreateService(service api.Service) error {
	registry.serviceData[service.ID] = service
	return nil
}

func (registry *MemoryRegistry) DeleteService(serviceID string) error {
	delete(registry.serviceData, serviceID)
	return nil
}

func (registry *MemoryRegistry) UpdateService(service api.Service) error {
	registry.serviceData[service.ID] = service
	return nil
}
