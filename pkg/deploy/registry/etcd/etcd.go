package etcd

import (
	"strconv"

	"github.com/golang/glog"

	etcderr "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors/etcd"

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
	// DeploymentConfigPath is the path to deploymentConfig resources in etcd
	DeploymentConfigPath string = "/deploymentConfigs"
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
func (r *Etcd) ListDeployments(ctx kapi.Context, label, field labels.Selector) (*api.DeploymentList, error) {
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
	err = r.CreateObj(key, deployment, 0)
	return etcderr.InterpretCreateError(err, "deployment", deployment.Name)
}

// UpdateDeployment replaces an existing Deployment.
func (r *Etcd) UpdateDeployment(ctx kapi.Context, deployment *api.Deployment) error {
	key, err := makeDeploymentKey(ctx, deployment.Name)
	if err != nil {
		return err
	}
	err = r.SetObj(key, deployment)
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
func (r *Etcd) WatchDeployments(ctx kapi.Context, label, field labels.Selector, resourceVersion string) (watch.Interface, error) {
	version, err := parseWatchResourceVersion(resourceVersion, "deployment")
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

// ListDeploymentConfigs obtains a list of DeploymentConfigs.
func (r *Etcd) ListDeploymentConfigs(ctx kapi.Context, label, field labels.Selector) (*api.DeploymentConfigList, error) {
	deploymentConfigs := api.DeploymentConfigList{}
	err := r.ExtractToList(makeDeploymentConfigListKey(ctx), &deploymentConfigs)
	if err != nil {
		return nil, err
	}
	filtered := []api.DeploymentConfig{}
	for _, item := range deploymentConfigs.Items {
		fields := labels.Set{
			"name": item.Name,
		}
		if label.Matches(labels.Set(item.Labels)) && field.Matches(fields) {
			filtered = append(filtered, item)
		}
	}

	deploymentConfigs.Items = filtered
	return &deploymentConfigs, err
}

// TODO expose this from kubernetes.  I will do that, but I don't want this merge stuck on kubernetes refactoring
// parseWatchResourceVersion takes a resource version argument and converts it to
// the etcd version we should pass to helper.Watch(). Because resourceVersion is
// an opaque value, the default watch behavior for non-zero watch is to watch
// the next value (if you pass "1", you will see updates from "2" onwards).
func parseWatchResourceVersion(resourceVersion, kind string) (uint64, error) {
	if resourceVersion == "" || resourceVersion == "0" {
		return 0, nil
	}
	version, err := strconv.ParseUint(resourceVersion, 10, 64)
	if err != nil {
		return 0, etcderr.InterpretResourceVersionError(err, kind, resourceVersion)
	}
	return version + 1, nil
}

// WatchDeploymentConfigs begins watching for new, changed, or deleted DeploymentConfigs.
func (r *Etcd) WatchDeploymentConfigs(ctx kapi.Context, label, field labels.Selector, resourceVersion string) (watch.Interface, error) {
	version, err := parseWatchResourceVersion(resourceVersion, "deploymentConfig")
	if err != nil {
		return nil, err
	}

	return r.WatchList(makeDeploymentConfigListKey(ctx), version, func(obj runtime.Object) bool {
		config, ok := obj.(*api.DeploymentConfig)
		if !ok {
			glog.Errorf("Unexpected object during deploymentConfig watch: %#v", obj)
			return false
		}
		fields := labels.Set{
			"name": config.Name,
		}
		return label.Matches(labels.Set(config.Labels)) && field.Matches(fields)
	})
}

func makeDeploymentConfigListKey(ctx kapi.Context) string {
	return kubeetcd.MakeEtcdListKey(ctx, DeploymentConfigPath)
}

func makeDeploymentConfigKey(ctx kapi.Context, id string) (string, error) {
	return kubeetcd.MakeEtcdItemKey(ctx, DeploymentConfigPath, id)
}

// GetDeploymentConfig gets a specific DeploymentConfig specified by its ID.
func (r *Etcd) GetDeploymentConfig(ctx kapi.Context, id string) (*api.DeploymentConfig, error) {
	var deploymentConfig api.DeploymentConfig
	key, err := makeDeploymentConfigKey(ctx, id)
	if err != nil {
		return nil, err
	}

	err = r.ExtractObj(key, &deploymentConfig, false)
	if err != nil {
		return nil, etcderr.InterpretGetError(err, "deploymentConfig", id)
	}
	return &deploymentConfig, nil
}

// CreateDeploymentConfig creates a new DeploymentConfig.
func (r *Etcd) CreateDeploymentConfig(ctx kapi.Context, deploymentConfig *api.DeploymentConfig) error {
	key, err := makeDeploymentConfigKey(ctx, deploymentConfig.Name)
	if err != nil {
		return err
	}

	err = r.CreateObj(key, deploymentConfig, 0)
	return etcderr.InterpretCreateError(err, "deploymentConfig", deploymentConfig.Name)
}

// UpdateDeploymentConfig replaces an existing DeploymentConfig.
func (r *Etcd) UpdateDeploymentConfig(ctx kapi.Context, deploymentConfig *api.DeploymentConfig) error {
	key, err := makeDeploymentConfigKey(ctx, deploymentConfig.Name)
	if err != nil {
		return err
	}

	err = r.SetObj(key, deploymentConfig)
	return etcderr.InterpretUpdateError(err, "deploymentConfig", deploymentConfig.Name)
}

// DeleteDeploymentConfig deletes a DeploymentConfig specified by its ID.
func (r *Etcd) DeleteDeploymentConfig(ctx kapi.Context, id string) error {
	key, err := makeDeploymentConfigKey(ctx, id)
	if err != nil {
		return err
	}

	err = r.Delete(key, false)
	return etcderr.InterpretDeleteError(err, "deploymentConfig", id)
}
