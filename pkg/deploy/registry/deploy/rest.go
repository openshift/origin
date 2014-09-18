package deploy

import (
	"fmt"

	"code.google.com/p/go-uuid/uuid"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kubeerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/golang/glog"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	"github.com/openshift/origin/pkg/deploy/api/validation"
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

// List obtains a list of Deployments that match selector.
func (s *REST) List(selector labels.Selector) (interface{}, error) {
	deployments, err := s.registry.ListDeployments(selector)
	if err != nil {
		return nil, err
	}

	return deployments, nil
}

// New creates a new Deployment for use with Create and Update
func (s *REST) New() interface{} {
	return &deployapi.Deployment{}
}

// Get obtains the Deployment specified by its id.
func (s *REST) Get(id string) (interface{}, error) {
	deployment, err := s.registry.GetDeployment(id)
	if err != nil {
		return nil, err
	}
	return deployment, err
}

// Delete asynchronously deletes the Deployment specified by its id.
func (s *REST) Delete(id string) (<-chan interface{}, error) {
	return apiserver.MakeAsync(func() (interface{}, error) {
		return api.Status{Status: api.StatusSuccess}, s.registry.DeleteDeployment(id)
	}), nil
}

// Create registers a given new Deployment instance to s.registry.
func (s *REST) Create(obj interface{}) (<-chan interface{}, error) {
	deployment, ok := obj.(*deployapi.Deployment)
	if !ok {
		return nil, fmt.Errorf("not a deployment: %#v", obj)
	}

	glog.Infof("Creating deployment with ID: %v", deployment.ID)

	if len(deployment.ID) == 0 {
		deployment.ID = uuid.NewUUID().String()
	}
	deployment.State = deployapi.DeploymentNew

	if errs := validation.ValidateDeployment(deployment); len(errs) > 0 {
		return nil, kubeerrors.NewInvalid("deployment", deployment.ID, errs)
	}

	return apiserver.MakeAsync(func() (interface{}, error) {
		err := s.registry.CreateDeployment(deployment)
		if err != nil {
			return nil, err
		}
		return *deployment, nil
	}), nil
}

// Update replaces a given Deployment instance with an existing instance in s.registry.
func (s *REST) Update(obj interface{}) (<-chan interface{}, error) {
	deployment, ok := obj.(*deployapi.Deployment)
	if !ok {
		return nil, fmt.Errorf("not a deployment: %#v", obj)
	}
	if len(deployment.ID) == 0 {
		return nil, fmt.Errorf("id is unspecified: %#v", deployment)
	}
	return apiserver.MakeAsync(func() (interface{}, error) {
		err := s.registry.UpdateDeployment(deployment)
		if err != nil {
			return nil, err
		}
		return deployment, nil
	}), nil
}
