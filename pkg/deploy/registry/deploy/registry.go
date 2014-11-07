package deploy

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
)

// Registry is an interface for things that know how to store Deployments.
type Registry interface {
	ListDeployments(ctx kapi.Context, label, field labels.Selector) (*deployapi.DeploymentList, error)
	GetDeployment(ctx kapi.Context, id string) (*deployapi.Deployment, error)
	CreateDeployment(ctx kapi.Context, deployment *deployapi.Deployment) error
	UpdateDeployment(ctx kapi.Context, deployment *deployapi.Deployment) error
	DeleteDeployment(ctx kapi.Context, id string) error
	WatchDeployments(ctx kapi.Context, label, field labels.Selector, resourceVersion string) (watch.Interface, error)
}
