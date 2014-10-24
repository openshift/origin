package deploy

import (
  "github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
  "github.com/GoogleCloudPlatform/kubernetes/pkg/watch"
  api "github.com/openshift/origin/pkg/deploy/api"
)

// Registry is an interface for things that know how to store Deployments.
type Registry interface {
  ListDeployments(selector labels.Selector) (*api.DeploymentList, error)
  GetDeployment(id string) (*api.Deployment, error)
  CreateDeployment(deployment *api.Deployment) error
  UpdateDeployment(deployment *api.Deployment) error
  DeleteDeployment(id string) error
  WatchDeployments(resourceVersion uint64, filter func(repo *api.Deployment) bool) (watch.Interface, error)
}
