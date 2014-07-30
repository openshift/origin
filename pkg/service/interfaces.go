package service

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/openshift/origin/pkg/api"
)

// ServiceRegistry is an interface implemented by things that know how to store Service objects.
type ServiceRegistry interface {
	// ListServices obtains a list of services that match selector.
	ListServices(selector labels.Selector) ([]api.Service, error)
	// Get a specific service
	GetService(serviceID string) (*api.Service, error)
	CreateService(service api.Service) error
	// Update an existing service
	UpdateService(service api.Service) error
	// Delete an existing service
	DeleteService(serviceID string) error
}
