package deploy

import (
	kubeapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"
	api "github.com/openshift/origin/pkg/deploy/api"
)

// Registry is an interface for things that know how to store Deployments.
type Registry interface {
	ListDeployments(ctx kubeapi.Context, selector labels.Selector) (*api.DeploymentList, error)
	GetDeployment(ctx kubeapi.Context, id string) (*api.Deployment, error)
	CreateDeployment(ctx kubeapi.Context, deployment *api.Deployment) error
	UpdateDeployment(ctx kubeapi.Context, deployment *api.Deployment) error
	DeleteDeployment(ctx kubeapi.Context, id string) error
	WatchDeployments(ctx kubeapi.Context, resourceVersion string, filter func(repo *api.Deployment) bool) (watch.Interface, error)
}
