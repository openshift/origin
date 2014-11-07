package test

import (
	"sync"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"
	"github.com/openshift/origin/pkg/deploy/api"
)

type DeploymentConfigRegistry struct {
	Err               error
	DeploymentConfig  *api.DeploymentConfig
	DeploymentConfigs *api.DeploymentConfigList
	sync.Mutex
}

func NewDeploymentConfigRegistry() *DeploymentConfigRegistry {
	return &DeploymentConfigRegistry{}
}

func (r *DeploymentConfigRegistry) ListDeploymentConfigs(ctx kapi.Context, label, field labels.Selector) (*api.DeploymentConfigList, error) {
	r.Lock()
	defer r.Unlock()

	return r.DeploymentConfigs, r.Err
}

func (r *DeploymentConfigRegistry) GetDeploymentConfig(ctx kapi.Context, id string) (*api.DeploymentConfig, error) {
	r.Lock()
	defer r.Unlock()

	return r.DeploymentConfig, r.Err
}

func (r *DeploymentConfigRegistry) CreateDeploymentConfig(ctx kapi.Context, image *api.DeploymentConfig) error {
	r.Lock()
	defer r.Unlock()

	r.DeploymentConfig = image
	return r.Err
}

func (r *DeploymentConfigRegistry) UpdateDeploymentConfig(ctx kapi.Context, image *api.DeploymentConfig) error {
	r.Lock()
	defer r.Unlock()

	r.DeploymentConfig = image
	return r.Err
}

func (r *DeploymentConfigRegistry) DeleteDeploymentConfig(ctx kapi.Context, id string) error {
	r.Lock()
	defer r.Unlock()

	return r.Err
}

func (r *DeploymentConfigRegistry) WatchDeploymentConfigs(ctx kapi.Context, label, field labels.Selector, resourceVersion string) (watch.Interface, error) {
	return nil, r.Err
}
