package deploy

import (
	"fmt"

	"code.google.com/p/go-uuid/uuid"
	kubeapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kubeerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"
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

// New creates a new Deployment for use with Create and Update
func (s *REST) New() runtime.Object {
	return &deployapi.Deployment{}
}

// List obtains a list of Deployments that match selector.
func (s *REST) List(ctx kubeapi.Context, selector, fields labels.Selector) (runtime.Object, error) {
	deployments, err := s.registry.ListDeployments(selector)
	if err != nil {
		return nil, err
	}

	return deployments, nil
}

// Get obtains the Deployment specified by its id.
func (s *REST) Get(ctx kubeapi.Context, id string) (runtime.Object, error) {
	deployment, err := s.registry.GetDeployment(id)
	if err != nil {
		return nil, err
	}
	return deployment, err
}

// Delete asynchronously deletes the Deployment specified by its id.
func (s *REST) Delete(ctx kubeapi.Context, id string) (<-chan runtime.Object, error) {
	return apiserver.MakeAsync(func() (runtime.Object, error) {
		return &kubeapi.Status{Status: kubeapi.StatusSuccess}, s.registry.DeleteDeployment(id)
	}), nil
}

// Create registers a given new Deployment instance to s.registry.
func (s *REST) Create(ctx kubeapi.Context, obj runtime.Object) (<-chan runtime.Object, error) {
	deployment, ok := obj.(*deployapi.Deployment)
	if !ok {
		return nil, fmt.Errorf("not a deployment: %#v", obj)
	}

	glog.Infof("Creating deployment with ID: %v", deployment.ID)

	if len(deployment.ID) == 0 {
		deployment.ID = uuid.NewUUID().String()
	}
	deployment.State = deployapi.DeploymentStateNew

	if errs := validation.ValidateDeployment(deployment); len(errs) > 0 {
		return nil, kubeerrors.NewInvalid("deployment", deployment.ID, errs)
	}

	return apiserver.MakeAsync(func() (runtime.Object, error) {
		err := s.registry.CreateDeployment(deployment)
		if err != nil {
			return nil, err
		}
		return deployment, nil
	}), nil
}

// Update replaces a given Deployment instance with an existing instance in s.registry.
func (s *REST) Update(ctx kubeapi.Context, obj runtime.Object) (<-chan runtime.Object, error) {
	deployment, ok := obj.(*deployapi.Deployment)
	if !ok {
		return nil, fmt.Errorf("not a deployment: %#v", obj)
	}
	if len(deployment.ID) == 0 {
		return nil, fmt.Errorf("id is unspecified: %#v", deployment)
	}
	return apiserver.MakeAsync(func() (runtime.Object, error) {
		err := s.registry.UpdateDeployment(deployment)
		if err != nil {
			return nil, err
		}
		return deployment, nil
	}), nil
}

// Watch begins watching for new, changed, or deleted Deployments.
func (s *REST) Watch(ctx kubeapi.Context, label, field labels.Selector, resourceVersion uint64) (watch.Interface, error) {
	return s.registry.WatchDeployments(resourceVersion, func(deployment *deployapi.Deployment) bool {
		fields := labels.Set{
			"ID": deployment.ID,
		}
		return label.Matches(labels.Set(deployment.Labels)) && field.Matches(fields)
	})
}
