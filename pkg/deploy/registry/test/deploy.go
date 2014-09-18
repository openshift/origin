package test

import (
	"sync"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
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
