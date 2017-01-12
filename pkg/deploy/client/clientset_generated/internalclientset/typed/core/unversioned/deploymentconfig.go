package unversioned

import (
	api "github.com/openshift/origin/pkg/deploy/api"
	pkg_api "k8s.io/kubernetes/pkg/api"
	watch "k8s.io/kubernetes/pkg/watch"
)

// DeploymentConfigsGetter has a method to return a DeploymentConfigInterface.
// A group's client should implement this interface.
type DeploymentConfigsGetter interface {
	DeploymentConfigs(namespace string) DeploymentConfigInterface
}

// DeploymentConfigInterface has methods to work with DeploymentConfig resources.
type DeploymentConfigInterface interface {
	Create(*api.DeploymentConfig) (*api.DeploymentConfig, error)
	Update(*api.DeploymentConfig) (*api.DeploymentConfig, error)
	Delete(name string, options *pkg_api.DeleteOptions) error
	DeleteCollection(options *pkg_api.DeleteOptions, listOptions pkg_api.ListOptions) error
	Get(name string) (*api.DeploymentConfig, error)
	List(opts pkg_api.ListOptions) (*api.DeploymentConfigList, error)
	Watch(opts pkg_api.ListOptions) (watch.Interface, error)
	Patch(name string, pt pkg_api.PatchType, data []byte, subresources ...string) (result *api.DeploymentConfig, err error)
	DeploymentConfigExpansion
}

// deploymentConfigs implements DeploymentConfigInterface
type deploymentConfigs struct {
	client *CoreClient
	ns     string
}

// newDeploymentConfigs returns a DeploymentConfigs
func newDeploymentConfigs(c *CoreClient, namespace string) *deploymentConfigs {
	return &deploymentConfigs{
		client: c,
		ns:     namespace,
	}
}

// Create takes the representation of a deploymentConfig and creates it.  Returns the server's representation of the deploymentConfig, and an error, if there is any.
func (c *deploymentConfigs) Create(deploymentConfig *api.DeploymentConfig) (result *api.DeploymentConfig, err error) {
	result = &api.DeploymentConfig{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("deploymentconfigs").
		Body(deploymentConfig).
		Do().
		Into(result)
	return
}

// Update takes the representation of a deploymentConfig and updates it. Returns the server's representation of the deploymentConfig, and an error, if there is any.
func (c *deploymentConfigs) Update(deploymentConfig *api.DeploymentConfig) (result *api.DeploymentConfig, err error) {
	result = &api.DeploymentConfig{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("deploymentconfigs").
		Name(deploymentConfig.Name).
		Body(deploymentConfig).
		Do().
		Into(result)
	return
}

// Delete takes name of the deploymentConfig and deletes it. Returns an error if one occurs.
func (c *deploymentConfigs) Delete(name string, options *pkg_api.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("deploymentconfigs").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *deploymentConfigs) DeleteCollection(options *pkg_api.DeleteOptions, listOptions pkg_api.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("deploymentconfigs").
		VersionedParams(&listOptions, pkg_api.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Get takes name of the deploymentConfig, and returns the corresponding deploymentConfig object, and an error if there is any.
func (c *deploymentConfigs) Get(name string) (result *api.DeploymentConfig, err error) {
	result = &api.DeploymentConfig{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("deploymentconfigs").
		Name(name).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of DeploymentConfigs that match those selectors.
func (c *deploymentConfigs) List(opts pkg_api.ListOptions) (result *api.DeploymentConfigList, err error) {
	result = &api.DeploymentConfigList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("deploymentconfigs").
		VersionedParams(&opts, pkg_api.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested deploymentConfigs.
func (c *deploymentConfigs) Watch(opts pkg_api.ListOptions) (watch.Interface, error) {
	return c.client.Get().
		Prefix("watch").
		Namespace(c.ns).
		Resource("deploymentconfigs").
		VersionedParams(&opts, pkg_api.ParameterCodec).
		Watch()
}

// Patch applies the patch and returns the patched deploymentConfig.
func (c *deploymentConfigs) Patch(name string, pt pkg_api.PatchType, data []byte, subresources ...string) (result *api.DeploymentConfig, err error) {
	result = &api.DeploymentConfig{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("deploymentconfigs").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
