package client

import (
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/watch"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
)

// DeploymentConfigsNamespacer has methods to work with DeploymentConfig resources in a namespace
type DeploymentConfigsNamespacer interface {
	DeploymentConfigs(namespace string) DeploymentConfigInterface
}

// DeploymentConfigInterface contains methods for working with DeploymentConfigs
type DeploymentConfigInterface interface {
	List(label labels.Selector, field fields.Selector) (*deployapi.DeploymentConfigList, error)
	Get(name string) (*deployapi.DeploymentConfig, error)
	Create(config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error)
	Update(config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error)
	Delete(name string) error
	Watch(label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error)
	Generate(name string) (*deployapi.DeploymentConfig, error)
	Rollback(config *deployapi.DeploymentConfigRollback) (*deployapi.DeploymentConfig, error)
}

// deploymentConfigs implements DeploymentConfigsNamespacer interface
type deploymentConfigs struct {
	r  *Client
	ns string
}

// newDeploymentConfigs returns a deploymentConfigs
func newDeploymentConfigs(c *Client, namespace string) *deploymentConfigs {
	return &deploymentConfigs{
		r:  c,
		ns: namespace,
	}
}

// List takes a label and field selectors, and returns the list of deploymentConfigs that match that selectors
func (c *deploymentConfigs) List(label labels.Selector, field fields.Selector) (result *deployapi.DeploymentConfigList, err error) {
	result = &deployapi.DeploymentConfigList{}
	err = c.r.Get().
		Namespace(c.ns).
		Resource("deploymentConfigs").
		LabelsSelectorParam(label).
		FieldsSelectorParam(field).
		Do().
		Into(result)
	return
}

// Get returns information about a particular deploymentConfig
func (c *deploymentConfigs) Get(name string) (result *deployapi.DeploymentConfig, err error) {
	result = &deployapi.DeploymentConfig{}
	err = c.r.Get().Namespace(c.ns).Resource("deploymentConfigs").Name(name).Do().Into(result)
	return
}

// Create creates a new deploymentConfig
func (c *deploymentConfigs) Create(deploymentConfig *deployapi.DeploymentConfig) (result *deployapi.DeploymentConfig, err error) {
	result = &deployapi.DeploymentConfig{}
	err = c.r.Post().Namespace(c.ns).Resource("deploymentConfigs").Body(deploymentConfig).Do().Into(result)
	return
}

// Update updates an existing deploymentConfig
func (c *deploymentConfigs) Update(deploymentConfig *deployapi.DeploymentConfig) (result *deployapi.DeploymentConfig, err error) {
	result = &deployapi.DeploymentConfig{}
	err = c.r.Put().Namespace(c.ns).Resource("deploymentConfigs").Name(deploymentConfig.Name).Body(deploymentConfig).Do().Into(result)
	return
}

// Delete deletes an existing deploymentConfig.
func (c *deploymentConfigs) Delete(name string) error {
	return c.r.Delete().Namespace(c.ns).Resource("deploymentConfigs").Name(name).Do().Error()
}

// Watch returns a watch.Interface that watches the requested deploymentConfigs.
func (c *deploymentConfigs) Watch(label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error) {
	return c.r.Get().
		Prefix("watch").
		Namespace(c.ns).
		Resource("deploymentConfigs").
		Param("resourceVersion", resourceVersion).
		LabelsSelectorParam(label).
		FieldsSelectorParam(field).
		Watch()
}

// Generate generates a new deploymentConfig for the given name.
func (c *deploymentConfigs) Generate(name string) (result *deployapi.DeploymentConfig, err error) {
	result = &deployapi.DeploymentConfig{}
	err = c.r.Get().Namespace(c.ns).Resource("generateDeploymentConfigs").Name(name).Do().Into(result)
	return
}

func (c *deploymentConfigs) Rollback(config *deployapi.DeploymentConfigRollback) (result *deployapi.DeploymentConfig, err error) {
	result = &deployapi.DeploymentConfig{}
	err = c.r.Post().
		Namespace(c.ns).
		Resource("deploymentConfigRollbacks").
		Body(config).
		Do().
		Into(result)
	return
}
