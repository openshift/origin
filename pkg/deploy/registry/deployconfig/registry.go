package deployconfig

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
)

// Registry is an interface for things that know how to store DeploymentConfigs.
type Registry interface {
	ListDeploymentConfigs(ctx kapi.Context, label labels.Selector, field fields.Selector) (*deployapi.DeploymentConfigList, error)
	WatchDeploymentConfigs(ctx kapi.Context, label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error)
	GetDeploymentConfig(ctx kapi.Context, id string) (*deployapi.DeploymentConfig, error)
	CreateDeploymentConfig(ctx kapi.Context, deploymentConfig *deployapi.DeploymentConfig) error
	UpdateDeploymentConfig(ctx kapi.Context, deploymentConfig *deployapi.DeploymentConfig) error
	DeleteDeploymentConfig(ctx kapi.Context, id string) error
}
