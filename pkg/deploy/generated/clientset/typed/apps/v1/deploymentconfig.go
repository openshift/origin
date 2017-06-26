package v1

import (
	v1 "github.com/openshift/origin/pkg/deploy/apis/apps/v1"
	scheme "github.com/openshift/origin/pkg/deploy/generated/clientset/scheme"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// DeploymentConfigsGetter has a method to return a DeploymentConfigInterface.
// A group's client should implement this interface.
type DeploymentConfigsGetter interface {
	DeploymentConfigs(namespace string) DeploymentConfigInterface
}

// DeploymentConfigInterface has methods to work with DeploymentConfig resources.
type DeploymentConfigInterface interface {
	Create(*v1.DeploymentConfig) (*v1.DeploymentConfig, error)
	Update(*v1.DeploymentConfig) (*v1.DeploymentConfig, error)
	UpdateStatus(*v1.DeploymentConfig) (*v1.DeploymentConfig, error)
	Delete(name string, options *meta_v1.DeleteOptions) error
	DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error
	Get(name string, options meta_v1.GetOptions) (*v1.DeploymentConfig, error)
	List(opts meta_v1.ListOptions) (*v1.DeploymentConfigList, error)
	Watch(opts meta_v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.DeploymentConfig, err error)
	DeploymentConfigExpansion
}

// deploymentConfigs implements DeploymentConfigInterface
type deploymentConfigs struct {
	client rest.Interface
	ns     string
}

// newDeploymentConfigs returns a DeploymentConfigs
func newDeploymentConfigs(c *AppsV1Client, namespace string) *deploymentConfigs {
	return &deploymentConfigs{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Create takes the representation of a deploymentConfig and creates it.  Returns the server's representation of the deploymentConfig, and an error, if there is any.
func (c *deploymentConfigs) Create(deploymentConfig *v1.DeploymentConfig) (result *v1.DeploymentConfig, err error) {
	result = &v1.DeploymentConfig{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("deploymentconfigs").
		Body(deploymentConfig).
		Do().
		Into(result)
	return
}

// Update takes the representation of a deploymentConfig and updates it. Returns the server's representation of the deploymentConfig, and an error, if there is any.
func (c *deploymentConfigs) Update(deploymentConfig *v1.DeploymentConfig) (result *v1.DeploymentConfig, err error) {
	result = &v1.DeploymentConfig{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("deploymentconfigs").
		Name(deploymentConfig.Name).
		Body(deploymentConfig).
		Do().
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclientstatus=false comment above the type to avoid generating UpdateStatus().

func (c *deploymentConfigs) UpdateStatus(deploymentConfig *v1.DeploymentConfig) (result *v1.DeploymentConfig, err error) {
	result = &v1.DeploymentConfig{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("deploymentconfigs").
		Name(deploymentConfig.Name).
		SubResource("status").
		Body(deploymentConfig).
		Do().
		Into(result)
	return
}

// Delete takes name of the deploymentConfig and deletes it. Returns an error if one occurs.
func (c *deploymentConfigs) Delete(name string, options *meta_v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("deploymentconfigs").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *deploymentConfigs) DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("deploymentconfigs").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Get takes name of the deploymentConfig, and returns the corresponding deploymentConfig object, and an error if there is any.
func (c *deploymentConfigs) Get(name string, options meta_v1.GetOptions) (result *v1.DeploymentConfig, err error) {
	result = &v1.DeploymentConfig{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("deploymentconfigs").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of DeploymentConfigs that match those selectors.
func (c *deploymentConfigs) List(opts meta_v1.ListOptions) (result *v1.DeploymentConfigList, err error) {
	result = &v1.DeploymentConfigList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("deploymentconfigs").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested deploymentConfigs.
func (c *deploymentConfigs) Watch(opts meta_v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("deploymentconfigs").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Patch applies the patch and returns the patched deploymentConfig.
func (c *deploymentConfigs) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.DeploymentConfig, err error) {
	result = &v1.DeploymentConfig{}
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
