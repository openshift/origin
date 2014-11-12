package deployconfig

import (
	"fmt"

	"code.google.com/p/go-uuid/uuid"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"
	"github.com/golang/glog"

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
func (s *REST) List(ctx kapi.Context, label, field labels.Selector) (runtime.Object, error) {
	deploymentConfigs, err := s.registry.ListDeploymentConfigs(ctx, label, field)
	if err != nil {
		return nil, err
	}

	return deploymentConfigs, nil
}

// Watch begins watching for new, changed, or deleted ImageRepositories.
func (s *REST) Watch(ctx kapi.Context, label, field labels.Selector, resourceVersion string) (watch.Interface, error) {
	return s.registry.WatchDeploymentConfigs(ctx, label, field, resourceVersion)
}

// Get obtains the DeploymentConfig specified by its id.
func (s *REST) Get(ctx kapi.Context, id string) (runtime.Object, error) {
	deploymentConfig, err := s.registry.GetDeploymentConfig(ctx, id)
	if err != nil {
		return nil, err
	}
	return deploymentConfig, err
}

// Delete asynchronously deletes the DeploymentConfig specified by its id.
func (s *REST) Delete(ctx kapi.Context, id string) (<-chan apiserver.RESTResult, error) {
	return apiserver.MakeAsync(func() (runtime.Object, error) {
		return &kapi.Status{Status: kapi.StatusSuccess}, s.registry.DeleteDeploymentConfig(ctx, id)
	}), nil
}

// Create registers a given new DeploymentConfig instance to s.registry.
func (s *REST) Create(ctx kapi.Context, obj runtime.Object) (<-chan apiserver.RESTResult, error) {
	deploymentConfig, ok := obj.(*deployapi.DeploymentConfig)
	if !ok {
		return nil, fmt.Errorf("not a deploymentConfig: %#v", obj)
	}

	deploymentConfig.CreationTimestamp = util.Now()

	if len(deploymentConfig.Name) == 0 {
		deploymentConfig.Name = uuid.NewUUID().String()
	}
	if !kapi.ValidNamespace(ctx, &deploymentConfig.ObjectMeta) {
		return nil, kerrors.NewConflict("deploymentConfig", deploymentConfig.Namespace, fmt.Errorf("DeploymentConfig.Namespace does not match the provided context"))
	}

	glog.Infof("Creating deploymentConfig with namespace::Name: %v::%v", deploymentConfig.Namespace, deploymentConfig.Name)

	if errs := validation.ValidateDeploymentConfig(deploymentConfig); len(errs) > 0 {
		return nil, kerrors.NewInvalid("deploymentConfig", deploymentConfig.Name, errs)
	}

	return apiserver.MakeAsync(func() (runtime.Object, error) {
		err := s.registry.CreateDeploymentConfig(ctx, deploymentConfig)
		if err != nil {
			return nil, err
		}
		return deploymentConfig, nil
	}), nil
}

// Update replaces a given DeploymentConfig instance with an existing instance in s.registry.
func (s *REST) Update(ctx kapi.Context, obj runtime.Object) (<-chan apiserver.RESTResult, error) {
	deploymentConfig, ok := obj.(*deployapi.DeploymentConfig)
	if !ok {
		return nil, fmt.Errorf("not a deploymentConfig: %#v", obj)
	}
	if len(deploymentConfig.Name) == 0 {
		return nil, fmt.Errorf("id is unspecified: %#v", deploymentConfig)
	}
	if !kapi.ValidNamespace(ctx, &deploymentConfig.ObjectMeta) {
		return nil, kerrors.NewConflict("deploymentConfig", deploymentConfig.Namespace, fmt.Errorf("DeploymentConfig.Namespace does not match the provided context"))
	}

	return apiserver.MakeAsync(func() (runtime.Object, error) {
		err := s.registry.UpdateDeploymentConfig(ctx, deploymentConfig)
		if err != nil {
			return nil, err
		}
		return deploymentConfig, nil
	}), nil
}
