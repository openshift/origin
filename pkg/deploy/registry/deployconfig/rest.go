package deployconfig

import (
	"fmt"

	"code.google.com/p/go-uuid/uuid"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
)

// REST is an implementation of RESTStorage for the api server.
type REST struct {
	registry Registry
}

func NewREST(registry Registry) apiserver.RESTStorage {
	return &REST{
		registry: registry,
	}
}

// List obtains a list of DeploymentConfigs that match selector.
func (s *REST) List(selector labels.Selector) (interface{}, error) {
	deploymentConfigs, err := s.registry.ListDeploymentConfigs(selector)
	if err != nil {
		return nil, err
	}

	return deploymentConfigs, nil
}

// Get obtains the DeploymentConfig specified by its id.
func (s *REST) Get(id string) (interface{}, error) {
	deploymentConfig, err := s.registry.GetDeploymentConfig(id)
	if err != nil {
		return nil, err
	}
	return deploymentConfig, err
}

// Delete asynchronously deletes the DeploymentConfig specified by its id.
func (s *REST) Delete(id string) (<-chan interface{}, error) {
	return apiserver.MakeAsync(func() (interface{}, error) {
		return api.Status{Status: api.StatusSuccess}, s.registry.DeleteDeploymentConfig(id)
	}), nil
}

// New creates a new DeploymentConfig for use with Create and Update
func (s *REST) New() interface{} {
	return &deployapi.DeploymentConfig{}
}

// Create registers a given new DeploymentConfig instance to s.registry.
func (s *REST) Create(obj interface{}) (<-chan interface{}, error) {
	deploymentConfig, ok := obj.(*deployapi.DeploymentConfig)
	if !ok {
		return nil, fmt.Errorf("not a deploymentConfig: %#v", obj)
	}
	if len(deploymentConfig.ID) == 0 {
		deploymentConfig.ID = uuid.NewUUID().String()
	}

	//TODO: Add validation

	return apiserver.MakeAsync(func() (interface{}, error) {
		err := s.registry.CreateDeploymentConfig(deploymentConfig)
		if err != nil {
			return nil, err
		}
		return deploymentConfig, nil
	}), nil
}

// Update replaces a given DeploymentConfig instance with an existing instance in s.registry.
func (s *REST) Update(obj interface{}) (<-chan interface{}, error) {
	deploymentConfig, ok := obj.(*deployapi.DeploymentConfig)
	if !ok {
		return nil, fmt.Errorf("not a deploymentConfig: %#v", obj)
	}
	if len(deploymentConfig.ID) == 0 {
		return nil, fmt.Errorf("id is unspecified: %#v", deploymentConfig)
	}
	return apiserver.MakeAsync(func() (interface{}, error) {
		err := s.registry.UpdateDeploymentConfig(deploymentConfig)
		if err != nil {
			return nil, err
		}
		return deploymentConfig, nil
	}), nil
}
