package deploy

import (
	"fmt"

	"code.google.com/p/go-uuid/uuid"
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
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

// NewREST creates a new REST backed by the given registry.
func NewREST(registry Registry) *REST {
	return &REST{
		registry: registry,
	}
}

// New creates a new Deployment for use with Create and Update.
func (s *REST) New() runtime.Object {
	return &deployapi.Deployment{}
}

func (*REST) NewList() runtime.Object {
	return &deployapi.Deployment{}
}

// List obtains a list of Deployments that match selector.
func (s *REST) List(ctx kapi.Context, label labels.Selector, field fields.Selector) (runtime.Object, error) {
	deployments, err := s.registry.ListDeployments(ctx, label, field)
	if err != nil {
		return nil, err
	}

	return deployments, nil
}

// Get obtains the Deployment specified by its id.
func (s *REST) Get(ctx kapi.Context, id string) (runtime.Object, error) {
	deployment, err := s.registry.GetDeployment(ctx, id)
	if err != nil {
		return nil, err
	}
	return deployment, err
}

// Delete asynchronously deletes the Deployment specified by its id.
func (s *REST) Delete(ctx kapi.Context, id string) (runtime.Object, error) {
	return &kapi.Status{Status: kapi.StatusSuccess}, s.registry.DeleteDeployment(ctx, id)
}

// Create registers a given new Deployment instance to s.registry.
func (s *REST) Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error) {
	deployment, ok := obj.(*deployapi.Deployment)
	if !ok {
		return nil, fmt.Errorf("not a deployment: %#v", obj)
	}
	if !kapi.ValidNamespace(ctx, &deployment.ObjectMeta) {
		return nil, kerrors.NewConflict("deployment", deployment.Namespace, fmt.Errorf("Deployment.Namespace does not match the provided context"))
	}

	kapi.FillObjectMetaSystemFields(ctx, &deployment.ObjectMeta)

	if len(deployment.Name) == 0 {
		deployment.Name = uuid.NewUUID().String()
	}
	deployment.Status = deployapi.DeploymentStatusNew

	glog.Infof("Creating deployment with namespace::Name: %v::%v", deployment.Namespace, deployment.Name)

	if errs := validation.ValidateDeployment(deployment); len(errs) > 0 {
		return nil, kerrors.NewInvalid("deployment", deployment.Name, errs)
	}

	err := s.registry.CreateDeployment(ctx, deployment)
	if err != nil {
		return nil, err
	}
	return deployment, nil
}

// Update replaces a given Deployment instance with an existing instance in s.registry.
func (s *REST) Update(ctx kapi.Context, obj runtime.Object) (runtime.Object, bool, error) {
	deployment, ok := obj.(*deployapi.Deployment)
	if !ok {
		return nil, false, fmt.Errorf("not a deployment: %#v", obj)
	}
	if len(deployment.Name) == 0 {
		return nil, false, fmt.Errorf("name is unspecified: %#v", deployment)
	}
	if !kapi.ValidNamespace(ctx, &deployment.ObjectMeta) {
		return nil, false, kerrors.NewConflict("deployment", deployment.Namespace, fmt.Errorf("Deployment.Namespace does not match the provided context"))
	}

	err := s.registry.UpdateDeployment(ctx, deployment)
	if err != nil {
		return nil, false, err
	}
	return deployment, false, nil
}

func (s *REST) Watch(ctx kapi.Context, label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error) {
	return s.registry.WatchDeployments(ctx, label, field, resourceVersion)
}
