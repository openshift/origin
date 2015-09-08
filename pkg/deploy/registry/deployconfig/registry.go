package deployconfig

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/rest"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/watch"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
)

// Registry is an interface for things that know how to store DeploymentConfigs.
type Registry interface {
	ListDeploymentConfigs(ctx kapi.Context, label labels.Selector, field fields.Selector) (*deployapi.DeploymentConfigList, error)
	WatchDeploymentConfigs(ctx kapi.Context, label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error)
	GetDeploymentConfig(ctx kapi.Context, name string) (*deployapi.DeploymentConfig, error)
	CreateDeploymentConfig(ctx kapi.Context, deploymentConfig *deployapi.DeploymentConfig) error
	UpdateDeploymentConfig(ctx kapi.Context, deploymentConfig *deployapi.DeploymentConfig) error
	DeleteDeploymentConfig(ctx kapi.Context, name string) error
}

// storage puts strong typing around storage calls
type storage struct {
	rest.StandardStorage
}

// NewRegistry returns a new Registry interface for the given Storage. Any mismatched
// types will panic.
func NewRegistry(s rest.StandardStorage) Registry {
	return &storage{s}
}

func (s *storage) ListDeploymentConfigs(ctx kapi.Context, label labels.Selector, field fields.Selector) (*deployapi.DeploymentConfigList, error) {
	obj, err := s.List(ctx, label, field)
	if err != nil {
		return nil, err
	}
	return obj.(*deployapi.DeploymentConfigList), nil
}

func (s *storage) WatchDeploymentConfigs(ctx kapi.Context, label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error) {
	return s.Watch(ctx, label, field, resourceVersion)
}

func (s *storage) GetDeploymentConfig(ctx kapi.Context, name string) (*deployapi.DeploymentConfig, error) {
	obj, err := s.Get(ctx, name)
	if err != nil {
		return nil, err
	}
	return obj.(*deployapi.DeploymentConfig), nil
}

func (s *storage) CreateDeploymentConfig(ctx kapi.Context, deploymentConfig *deployapi.DeploymentConfig) error {
	_, err := s.Create(ctx, deploymentConfig)
	return err
}

func (s *storage) UpdateDeploymentConfig(ctx kapi.Context, deploymentConfig *deployapi.DeploymentConfig) error {
	_, _, err := s.Update(ctx, deploymentConfig)
	return err
}

func (s *storage) DeleteDeploymentConfig(ctx kapi.Context, name string) error {
	_, err := s.Delete(ctx, name, nil)
	return err
}
