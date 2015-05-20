package etcd

import (
	"github.com/golang/glog"

	etcderr "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors/etcd"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	kubeetcd "github.com/GoogleCloudPlatform/kubernetes/pkg/registry/etcd"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/tools"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	"github.com/openshift/origin/pkg/deploy/api"
)

const (
	// DeploymentPath is the path to deployment resources in etcd
	DeploymentPath string = "/deployments"
)

// Etcd implements deployment.Registry and deploymentconfig.Registry interfaces.
type Etcd struct {
	tools.EtcdHelper
}

// New creates an etcd registry.
func New(helper tools.EtcdHelper) *Etcd {
	return &Etcd{
		EtcdHelper: helper,
	}
}

// ListDeployments obtains a list of Deployments.
func (r *Etcd) ListDeployments(ctx kapi.Context, label labels.Selector, field fields.Selector) (*api.DeploymentList, error) {
	deployments := api.DeploymentList{}
	err := r.ExtractToList(makeDeploymentListKey(ctx), &deployments)
	if err != nil {
		return nil, err
	}

	filtered := []api.Deployment{}
	for _, item := range deployments.Items {
		fields := labels.Set{
			"name":   item.Name,
			"status": string(item.Status),
		}
		if label.Matches(labels.Set(item.Labels)) && field.Matches(fields) {
			filtered = append(filtered, item)
		}
	}

	deployments.Items = filtered
	return &deployments, err
}

func makeDeploymentListKey(ctx kapi.Context) string {
	return kubeetcd.MakeEtcdListKey(ctx, DeploymentPath)
}

func makeDeploymentKey(ctx kapi.Context, id string) (string, error) {
	return kubeetcd.MakeEtcdItemKey(ctx, DeploymentPath, id)
}

// GetDeployment gets a specific Deployment specified by its ID.
func (r *Etcd) GetDeployment(ctx kapi.Context, id string) (*api.Deployment, error) {
	var deployment api.Deployment
	key, err := makeDeploymentKey(ctx, id)
	if err != nil {
		return nil, err
	}
	err = r.ExtractObj(key, &deployment, false)
	if err != nil {
		return nil, etcderr.InterpretGetError(err, "deployment", id)
	}
	return &deployment, nil
}

// CreateDeployment creates a new Deployment.
func (r *Etcd) CreateDeployment(ctx kapi.Context, deployment *api.Deployment) error {
	key, err := makeDeploymentKey(ctx, deployment.Name)
	if err != nil {
		return err
	}
	err = r.CreateObj(key, deployment, nil, 0)
	return etcderr.InterpretCreateError(err, "deployment", deployment.Name)
}

// UpdateDeployment replaces an existing Deployment.
func (r *Etcd) UpdateDeployment(ctx kapi.Context, deployment *api.Deployment) error {
	key, err := makeDeploymentKey(ctx, deployment.Name)
	if err != nil {
		return err
	}
	err = r.SetObj(key, deployment, nil, 0)
	return etcderr.InterpretUpdateError(err, "deployment", deployment.Name)
}

// DeleteDeployment deletes a Deployment specified by its ID.
func (r *Etcd) DeleteDeployment(ctx kapi.Context, id string) error {
	key, err := makeDeploymentKey(ctx, id)
	if err != nil {
		return err
	}
	err = r.Delete(key, false)
	return etcderr.InterpretDeleteError(err, "deployment", id)
}

// WatchDeployments begins watching for new, changed, or deleted Deployments.
func (r *Etcd) WatchDeployments(ctx kapi.Context, label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error) {
	version, err := tools.ParseWatchResourceVersion(resourceVersion, "deployment")
	if err != nil {
		return nil, err
	}

	return r.WatchList(makeDeploymentListKey(ctx), version, func(obj runtime.Object) bool {
		deployment, ok := obj.(*api.Deployment)
		if !ok {
			glog.Errorf("Unexpected object during deployment watch: %#v", obj)
			return false
		}
		fields := labels.Set{
			"name":   deployment.Name,
			"status": string(deployment.Status),
		}
		return label.Matches(labels.Set(deployment.Labels)) && field.Matches(fields)
	})
}
