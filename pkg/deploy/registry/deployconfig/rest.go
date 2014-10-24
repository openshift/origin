package deployconfig

import (
	"fmt"

	"code.google.com/p/go-uuid/uuid"
	kubeapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kubeerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	validation "github.com/openshift/origin/pkg/deploy/api/validation"
)

// REST is an implementation of RESTStorage for the api server.
type REST struct {
	registry Registry
}

// NewREST creates a new REST backed by the given registry.
func NewREST(registry Registry) apiserver.RESTStorage {
	return &REST{
		registry: registry,
	}
}

// New creates a new DeploymentConfig for use with Create and Update.
func (s *REST) New() runtime.Object {
	return &deployapi.DeploymentConfig{}
}

// List obtains a list of DeploymentConfigs that match selector.
func (s *REST) List(ctx kubeapi.Context, selector, fields labels.Selector) (runtime.Object, error) {
	deploymentConfigs, err := s.registry.ListDeploymentConfigs(selector)
	if err != nil {
		return nil, err
	}

	return deploymentConfigs, nil
}

// Watch begins watching for new, changed, or deleted ImageRepositories.
func (s *REST) Watch(ctx kubeapi.Context, label, field labels.Selector, resourceVersion uint64) (watch.Interface, error) {
	return s.registry.WatchDeploymentConfigs(resourceVersion, func(config *deployapi.DeploymentConfig) bool {
		fields := labels.Set{
			"ID": config.ID,
		}
		return label.Matches(labels.Set(config.Labels)) && field.Matches(fields)
	})
}

// Get obtains the DeploymentConfig specified by its id.
func (s *REST) Get(ctx kubeapi.Context, id string) (runtime.Object, error) {
	deploymentConfig, err := s.registry.GetDeploymentConfig(id)
	if err != nil {
		return nil, err
	}
	return deploymentConfig, err
}

// Delete asynchronously deletes the DeploymentConfig specified by its id.
func (s *REST) Delete(ctx kubeapi.Context, id string) (<-chan runtime.Object, error) {
	return apiserver.MakeAsync(func() (runtime.Object, error) {
		return &kubeapi.Status{Status: kubeapi.StatusSuccess}, s.registry.DeleteDeploymentConfig(id)
	}), nil
}

// Create registers a given new DeploymentConfig instance to s.registry.
func (s *REST) Create(ctx kubeapi.Context, obj runtime.Object) (<-chan runtime.Object, error) {
	deploymentConfig, ok := obj.(*deployapi.DeploymentConfig)
	if !ok {
		return nil, fmt.Errorf("not a deploymentConfig: %#v", obj)
	}
	if len(deploymentConfig.ID) == 0 {
		deploymentConfig.ID = uuid.NewUUID().String()
	}

	if errs := validation.ValidateDeploymentConfig(deploymentConfig); len(errs) > 0 {
		return nil, kubeerrors.NewInvalid("deploymentConfig", deploymentConfig.ID, errs)
	}

	return apiserver.MakeAsync(func() (runtime.Object, error) {
		err := s.registry.CreateDeploymentConfig(deploymentConfig)
		if err != nil {
			return nil, err
		}
		return deploymentConfig, nil
	}), nil
}

// Update replaces a given DeploymentConfig instance with an existing instance in s.registry.
func (s *REST) Update(ctx kubeapi.Context, obj runtime.Object) (<-chan runtime.Object, error) {
	deploymentConfig, ok := obj.(*deployapi.DeploymentConfig)
	if !ok {
		return nil, fmt.Errorf("not a deploymentConfig: %#v", obj)
	}
	if len(deploymentConfig.ID) == 0 {
		return nil, fmt.Errorf("id is unspecified: %#v", deploymentConfig)
	}
	return apiserver.MakeAsync(func() (runtime.Object, error) {
		err := s.registry.UpdateDeploymentConfig(deploymentConfig)
		if err != nil {
			return nil, err
		}
		return deploymentConfig, nil
	}), nil
}
