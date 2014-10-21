package test

import (
	"sync"

	kubeapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"
	"github.com/openshift/origin/pkg/deploy/api"
)

type DeploymentRegistry struct {
	Err         error
	Deployment  *api.Deployment
	Deployments *api.DeploymentList
	sync.Mutex
}

func NewDeploymentRegistry() *DeploymentRegistry {
	return &DeploymentRegistry{}
}

func (r *DeploymentRegistry) ListDeployments(ctx kubeapi.Context, selector labels.Selector) (*api.DeploymentList, error) {
	r.Lock()
	defer r.Unlock()

	return r.Deployments, r.Err
}

func (r *DeploymentRegistry) GetDeployment(ctx kubeapi.Context, id string) (*api.Deployment, error) {
	r.Lock()
	defer r.Unlock()

	return r.Deployment, r.Err
}

func (r *DeploymentRegistry) CreateDeployment(ctx kubeapi.Context, deployment *api.Deployment) error {
	r.Lock()
	defer r.Unlock()

	r.Deployment = deployment
	return r.Err
}

func (r *DeploymentRegistry) UpdateDeployment(ctx kubeapi.Context, deployment *api.Deployment) error {
	r.Lock()
	defer r.Unlock()

	r.Deployment = deployment
	return r.Err
}

func (r *DeploymentRegistry) DeleteDeployment(ctx kubeapi.Context, id string) error {
	r.Lock()
	defer r.Unlock()

	return r.Err
}

func (r *DeploymentRegistry) WatchDeployments(ctx kubeapi.Context, resourceVersion string, filter func(repo *api.Deployment) bool) (watch.Interface, error) {
	return nil, r.Err
}
