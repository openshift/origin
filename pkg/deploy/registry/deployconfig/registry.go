package deployconfig

import (
	kubeapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"
	api "github.com/openshift/origin/pkg/deploy/api"
)

// Registry is an interface for things that know how to store DeploymentConfigs.
type Registry interface {
	ListDeploymentConfigs(ctx kubeapi.Context, selector labels.Selector) (*api.DeploymentConfigList, error)
	WatchDeploymentConfigs(ctx kubeapi.Context, resourceVersion string, filter func(repo *api.DeploymentConfig) bool) (watch.Interface, error)
	GetDeploymentConfig(ctx kubeapi.Context, id string) (*api.DeploymentConfig, error)
	CreateDeploymentConfig(ctx kubeapi.Context, deploymentConfig *api.DeploymentConfig) error
	UpdateDeploymentConfig(ctx kubeapi.Context, deploymentConfig *api.DeploymentConfig) error
	DeleteDeploymentConfig(ctx kubeapi.Context, id string) error
}
