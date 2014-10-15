package test

import (
	"sync"

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

func (r *DeploymentRegistry) ListDeployments(selector labels.Selector) (*api.DeploymentList, error) {
	r.Lock()
	defer r.Unlock()

	return r.Deployments, r.Err
}

func (r *DeploymentRegistry) GetDeployment(id string) (*api.Deployment, error) {
	r.Lock()
	defer r.Unlock()

	return r.Deployment, r.Err
}

func (r *DeploymentRegistry) CreateDeployment(deployment *api.Deployment) error {
	r.Lock()
	defer r.Unlock()

	r.Deployment = deployment
	return r.Err
}

func (r *DeploymentRegistry) UpdateDeployment(deployment *api.Deployment) error {
	r.Lock()
	defer r.Unlock()

	r.Deployment = deployment
	return r.Err
}

func (r *DeploymentRegistry) DeleteDeployment(id string) error {
	r.Lock()
	defer r.Unlock()

	return r.Err
}

func (r *DeploymentRegistry) WatchDeployments(resourceVersion uint64, filter func(repo *api.Deployment) bool) (watch.Interface, error) {
	return nil, r.Err
}
