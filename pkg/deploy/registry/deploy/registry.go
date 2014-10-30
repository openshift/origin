package deploy

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"
	api "github.com/openshift/origin/pkg/deploy/api"
)

// Registry is an interface for things that know how to store Deployments.
type Registry interface {
	ListDeployments(ctx kapi.Context, selector labels.Selector) (*api.DeploymentList, error)
	GetDeployment(ctx kapi.Context, id string) (*api.Deployment, error)
	CreateDeployment(ctx kapi.Context, deployment *api.Deployment) error
	UpdateDeployment(ctx kapi.Context, deployment *api.Deployment) error
	DeleteDeployment(ctx kapi.Context, id string) error
	WatchDeployments(ctx kapi.Context, resourceVersion string, filter func(repo *api.Deployment) bool) (watch.Interface, error)
}
