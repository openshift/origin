package deployconfig

import (
	metainternal "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	kapi "k8s.io/kubernetes/pkg/api"

	deployapi "github.com/openshift/origin/pkg/deploy/apis/apps"
)

// Registry is an interface for things that know how to store DeploymentConfigs.
type Registry interface {
	ListDeploymentConfigs(ctx apirequest.Context, options *metainternal.ListOptions) (*deployapi.DeploymentConfigList, error)
	WatchDeploymentConfigs(ctx apirequest.Context, options *metainternal.ListOptions) (watch.Interface, error)
	GetDeploymentConfig(ctx apirequest.Context, name string, options *metav1.GetOptions) (*deployapi.DeploymentConfig, error)
	CreateDeploymentConfig(ctx apirequest.Context, deploymentConfig *deployapi.DeploymentConfig) error
	UpdateDeploymentConfig(ctx apirequest.Context, deploymentConfig *deployapi.DeploymentConfig) error
	DeleteDeploymentConfig(ctx apirequest.Context, name string) error
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

func (s *storage) ListDeploymentConfigs(ctx apirequest.Context, options *metainternal.ListOptions) (*deployapi.DeploymentConfigList, error) {
	obj, err := s.List(ctx, options)
	if err != nil {
		return nil, err
	}
	return obj.(*deployapi.DeploymentConfigList), nil
}

func (s *storage) WatchDeploymentConfigs(ctx apirequest.Context, options *metainternal.ListOptions) (watch.Interface, error) {
	return s.Watch(ctx, options)
}

func (s *storage) GetDeploymentConfig(ctx apirequest.Context, name string, options *metav1.GetOptions) (*deployapi.DeploymentConfig, error) {
	obj, err := s.Get(ctx, name, options)
	if err != nil {
		return nil, err
	}
	return obj.(*deployapi.DeploymentConfig), nil
}

func (s *storage) CreateDeploymentConfig(ctx apirequest.Context, deploymentConfig *deployapi.DeploymentConfig) error {
	_, err := s.Create(ctx, deploymentConfig, false)
	return err
}

func (s *storage) UpdateDeploymentConfig(ctx apirequest.Context, deploymentConfig *deployapi.DeploymentConfig) error {
	_, _, err := s.Update(ctx, deploymentConfig.Name, rest.DefaultUpdatedObjectInfo(deploymentConfig, kapi.Scheme))
	return err
}

func (s *storage) DeleteDeploymentConfig(ctx apirequest.Context, name string) error {
	_, _, err := s.Delete(ctx, name, nil)
	return err
}
