package deployconfig

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"
	api "github.com/openshift/origin/pkg/deploy/api"
)

// Registry is an interface for things that know how to store DeploymentConfigs.
type Registry interface {
	ListDeploymentConfigs(ctx kapi.Context, selector labels.Selector) (*api.DeploymentConfigList, error)
	WatchDeploymentConfigs(ctx kapi.Context, resourceVersion string, filter func(repo *api.DeploymentConfig) bool) (watch.Interface, error)
	GetDeploymentConfig(ctx kapi.Context, id string) (*api.DeploymentConfig, error)
	CreateDeploymentConfig(ctx kapi.Context, deploymentConfig *api.DeploymentConfig) error
	UpdateDeploymentConfig(ctx kapi.Context, deploymentConfig *api.DeploymentConfig) error
	DeleteDeploymentConfig(ctx kapi.Context, id string) error
}
