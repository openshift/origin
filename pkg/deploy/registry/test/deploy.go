package test

import (
	"sync"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
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

func (r *DeploymentRegistry) ListDeployments(ctx kapi.Context, label, field labels.Selector) (*api.DeploymentList, error) {
	r.Lock()
	defer r.Unlock()

	return r.Deployments, r.Err
}

func (r *DeploymentRegistry) GetDeployment(ctx kapi.Context, id string) (*api.Deployment, error) {
	r.Lock()
	defer r.Unlock()

	return r.Deployment, r.Err
}

func (r *DeploymentRegistry) CreateDeployment(ctx kapi.Context, deployment *api.Deployment) error {
	r.Lock()
	defer r.Unlock()

	r.Deployment = deployment
	return r.Err
}

func (r *DeploymentRegistry) UpdateDeployment(ctx kapi.Context, deployment *api.Deployment) error {
	r.Lock()
	defer r.Unlock()

	r.Deployment = deployment
	return r.Err
}

func (r *DeploymentRegistry) DeleteDeployment(ctx kapi.Context, id string) error {
	r.Lock()
	defer r.Unlock()

	return r.Err
}

func (r *DeploymentRegistry) WatchDeployments(ctx kapi.Context, label, field labels.Selector, resourceVersion string) (watch.Interface, error) {
	return nil, r.Err
}
