package deployconfig

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"
	api "github.com/openshift/origin/pkg/deploy/api"
)

// Registry is an interface for things that know how to store DeploymentConfigs
type Registry interface {
	ListDeploymentConfigs(selector labels.Selector) (*api.DeploymentConfigList, error)
	WatchDeploymentConfigs(resourceVersion uint64, filter func(repo *api.DeploymentConfig) bool) (watch.Interface, error)
	GetDeploymentConfig(id string) (*api.DeploymentConfig, error)
	CreateDeploymentConfig(deploymentConfig *api.DeploymentConfig) error
	UpdateDeploymentConfig(deploymentConfig *api.DeploymentConfig) error
	DeleteDeploymentConfig(id string) error
}
