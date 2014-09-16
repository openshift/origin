package deployconfig

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	api "github.com/openshift/origin/pkg/deploy/api"
)

// Registry is an interface for things that know how to store DeploymentConfigs
type Registry interface {
	ListDeploymentConfigs(selector labels.Selector) (*api.DeploymentConfigList, error)
	GetDeploymentConfig(id string) (*api.DeploymentConfig, error)
	CreateDeploymentConfig(deploymentConfig *api.DeploymentConfig) error
	UpdateDeploymentConfig(deploymentConfig *api.DeploymentConfig) error
	DeleteDeploymentConfig(id string) error
}
