package test

import (
	"sync"

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

func (r *DeploymentConfigRegistry) ListDeploymentConfigs(selector labels.Selector) (*api.DeploymentConfigList, error) {
	r.Lock()
	defer r.Unlock()

	return r.DeploymentConfigs, r.Err
}

func (r *DeploymentConfigRegistry) GetDeploymentConfig(id string) (*api.DeploymentConfig, error) {
	r.Lock()
	defer r.Unlock()

	return r.DeploymentConfig, r.Err
}

func (r *DeploymentConfigRegistry) CreateDeploymentConfig(image *api.DeploymentConfig) error {
	r.Lock()
	defer r.Unlock()

	r.DeploymentConfig = image
	return r.Err
}

func (r *DeploymentConfigRegistry) UpdateDeploymentConfig(image *api.DeploymentConfig) error {
	r.Lock()
	defer r.Unlock()

	r.DeploymentConfig = image
	return r.Err
}

func (r *DeploymentConfigRegistry) DeleteDeploymentConfig(id string) error {
	r.Lock()
	defer r.Unlock()

	return r.Err
}

func (r *DeploymentConfigRegistry) WatchDeploymentConfigs(resourceVersion uint64, filter func(repo *api.DeploymentConfig) bool) (watch.Interface, error) {
	return nil, r.Err
}
