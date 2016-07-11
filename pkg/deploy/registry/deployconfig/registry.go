package deployconfig

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/rest"
	"k8s.io/kubernetes/pkg/watch"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
)

// Registry is an interface for things that know how to store DeploymentConfigs.
type Registry interface {
	ListDeploymentConfigs(ctx kapi.Context, options *kapi.ListOptions) (*deployapi.DeploymentConfigList, error)
	WatchDeploymentConfigs(ctx kapi.Context, options *kapi.ListOptions) (watch.Interface, error)
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

func (s *storage) ListDeploymentConfigs(ctx kapi.Context, options *kapi.ListOptions) (*deployapi.DeploymentConfigList, error) {
	obj, err := s.List(ctx, options)
	if err != nil {
		return nil, err
	}
	return obj.(*deployapi.DeploymentConfigList), nil
}

func (s *storage) WatchDeploymentConfigs(ctx kapi.Context, options *kapi.ListOptions) (watch.Interface, error) {
	return s.Watch(ctx, options)
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
	_, _, err := s.Update(ctx, deploymentConfig.Name, rest.DefaultUpdatedObjectInfo(deploymentConfig, kapi.Scheme))
	return err
}

func (s *storage) DeleteDeploymentConfig(ctx kapi.Context, name string) error {
	_, err := s.Delete(ctx, name, nil)
	return err
}
