package etcd

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/tools"
	"github.com/openshift/origin/pkg/deploy/api"
)

// Etcd implements deployment.Registry and deploymentconfig.Registry interfaces.
type Etcd struct {
	tools.EtcdHelper
}

// NewEtcd creates an etcd registry using the given etcd client.
func NewEtcd(client tools.EtcdClient) *Etcd {
	registry := &Etcd{
		tools.EtcdHelper{client, runtime.Codec, runtime.ResourceVersioner},
	}
	return registry
}

// ListDeployments obtains a list of Deployments.
func (r *Etcd) ListDeployments(selector labels.Selector) (*api.DeploymentList, error) {
	deployments := api.DeploymentList{}
	err := r.ExtractList("/deployments", &deployments.Items, &deployments.ResourceVersion)
	if err != nil {
		return nil, err
	}
	filtered := []api.Deployment{}
	for _, item := range deployments.Items {
		if selector.Matches(labels.Set(item.Labels)) {
			filtered = append(filtered, item)
		}
	}

	deployments.Items = filtered
	return &deployments, err
}

func makeDeploymentKey(id string) string {
	return "/deployments/" + id
}

// GetDeployment gets a specific Deployment specified by its ID.
func (r *Etcd) GetDeployment(id string) (*api.Deployment, error) {
	var deployment api.Deployment
	key := makeDeploymentKey(id)
	err := r.ExtractObj(key, &deployment, false)
	if tools.IsEtcdNotFound(err) {
		return nil, errors.NewNotFound("deployment", id)
	}
	if err != nil {
		return nil, err
	}
	return &deployment, nil
}

// CreateDeployment creates a new Deployment.
func (r *Etcd) CreateDeployment(deployment *api.Deployment) error {
	err := r.CreateObj(makeDeploymentKey(deployment.ID), deployment)
	if tools.IsEtcdNodeExist(err) {
		return errors.NewAlreadyExists("deployment", deployment.ID)
	}
	return err
}

// UpdateDeployment replaces an existing Deployment.
func (r *Etcd) UpdateDeployment(deployment *api.Deployment) error {
	return r.SetObj(makeDeploymentKey(deployment.ID), deployment)
}

// DeleteDeployment deletes a Deployment specified by its ID.
func (r *Etcd) DeleteDeployment(id string) error {
	key := makeDeploymentKey(id)
	err := r.Delete(key, false)
	if tools.IsEtcdNotFound(err) {
		return errors.NewNotFound("deployment", id)
	}
	return err
}

// ListDeploymentConfigs obtains a list of DeploymentConfigs.
func (r *Etcd) ListDeploymentConfigs(selector labels.Selector) (*api.DeploymentConfigList, error) {
	deploymentConfigs := api.DeploymentConfigList{}
	err := r.ExtractList("/deploymentConfigs", &deploymentConfigs.Items, &deploymentConfigs.ResourceVersion)
	if err != nil {
		return nil, err
	}
	filtered := []api.DeploymentConfig{}
	for _, item := range deploymentConfigs.Items {
		if selector.Matches(labels.Set(item.Labels)) {
			filtered = append(filtered, item)
		}
	}

	deploymentConfigs.Items = filtered
	return &deploymentConfigs, err
}

func makeDeploymentConfigKey(id string) string {
	return "/deploymentConfigs/" + id
}

// GetDeploymentConfig gets a specific DeploymentConfig specified by its ID.
func (r *Etcd) GetDeploymentConfig(id string) (*api.DeploymentConfig, error) {
	var deploymentConfig api.DeploymentConfig
	key := makeDeploymentConfigKey(id)
	err := r.ExtractObj(key, &deploymentConfig, false)
	if tools.IsEtcdNotFound(err) {
		return nil, errors.NewNotFound("deploymentConfig", id)
	}
	if err != nil {
		return nil, err
	}
	return &deploymentConfig, nil
}

// CreateDeploymentConfig creates a new DeploymentConfig.
func (r *Etcd) CreateDeploymentConfig(deploymentConfig *api.DeploymentConfig) error {
	err := r.CreateObj(makeDeploymentConfigKey(deploymentConfig.ID), deploymentConfig)
	if tools.IsEtcdNodeExist(err) {
		return errors.NewAlreadyExists("deploymentConfig", deploymentConfig.ID)
	}
	return err
}

// UpdateDeploymentConfig replaces an existing DeploymentConfig.
func (r *Etcd) UpdateDeploymentConfig(deploymentConfig *api.DeploymentConfig) error {
	return r.SetObj(makeDeploymentConfigKey(deploymentConfig.ID), deploymentConfig)
}

// DeleteDeploymentConfig deletes a DeploymentConfig specified by its ID.
func (r *Etcd) DeleteDeploymentConfig(id string) error {
	key := makeDeploymentConfigKey(id)
	err := r.Delete(key, false)
	if tools.IsEtcdNotFound(err) {
		return errors.NewNotFound("deploymentConfig", id)
	}
	return err
}
