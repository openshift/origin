package v1

import (
	v1 "github.com/openshift/origin/pkg/cmd/openshift-operators/controller-operator/apis/controller/v1"
	scheme "github.com/openshift/origin/pkg/cmd/openshift-operators/controller-operator/generated/clientset/versioned/scheme"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// OpenShiftControllerConfigsGetter has a method to return a OpenShiftControllerConfigInterface.
// A group's client should implement this interface.
type OpenShiftControllerConfigsGetter interface {
	OpenShiftControllerConfigs() OpenShiftControllerConfigInterface
}

// OpenShiftControllerConfigInterface has methods to work with OpenShiftControllerConfig resources.
type OpenShiftControllerConfigInterface interface {
	Create(*v1.OpenShiftControllerConfig) (*v1.OpenShiftControllerConfig, error)
	Update(*v1.OpenShiftControllerConfig) (*v1.OpenShiftControllerConfig, error)
	UpdateStatus(*v1.OpenShiftControllerConfig) (*v1.OpenShiftControllerConfig, error)
	Delete(name string, options *meta_v1.DeleteOptions) error
	DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error
	Get(name string, options meta_v1.GetOptions) (*v1.OpenShiftControllerConfig, error)
	List(opts meta_v1.ListOptions) (*v1.OpenShiftControllerConfigList, error)
	Watch(opts meta_v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.OpenShiftControllerConfig, err error)
	OpenShiftControllerConfigExpansion
}

// openShiftControllerConfigs implements OpenShiftControllerConfigInterface
type openShiftControllerConfigs struct {
	client rest.Interface
}

// newOpenShiftControllerConfigs returns a OpenShiftControllerConfigs
func newOpenShiftControllerConfigs(c *ControllerV1Client) *openShiftControllerConfigs {
	return &openShiftControllerConfigs{
		client: c.RESTClient(),
	}
}

// Get takes name of the openShiftControllerConfig, and returns the corresponding openShiftControllerConfig object, and an error if there is any.
func (c *openShiftControllerConfigs) Get(name string, options meta_v1.GetOptions) (result *v1.OpenShiftControllerConfig, err error) {
	result = &v1.OpenShiftControllerConfig{}
	err = c.client.Get().
		Resource("openshiftcontrollerconfigs").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of OpenShiftControllerConfigs that match those selectors.
func (c *openShiftControllerConfigs) List(opts meta_v1.ListOptions) (result *v1.OpenShiftControllerConfigList, err error) {
	result = &v1.OpenShiftControllerConfigList{}
	err = c.client.Get().
		Resource("openshiftcontrollerconfigs").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested openShiftControllerConfigs.
func (c *openShiftControllerConfigs) Watch(opts meta_v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Resource("openshiftcontrollerconfigs").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Create takes the representation of a openShiftControllerConfig and creates it.  Returns the server's representation of the openShiftControllerConfig, and an error, if there is any.
func (c *openShiftControllerConfigs) Create(openShiftControllerConfig *v1.OpenShiftControllerConfig) (result *v1.OpenShiftControllerConfig, err error) {
	result = &v1.OpenShiftControllerConfig{}
	err = c.client.Post().
		Resource("openshiftcontrollerconfigs").
		Body(openShiftControllerConfig).
		Do().
		Into(result)
	return
}

// Update takes the representation of a openShiftControllerConfig and updates it. Returns the server's representation of the openShiftControllerConfig, and an error, if there is any.
func (c *openShiftControllerConfigs) Update(openShiftControllerConfig *v1.OpenShiftControllerConfig) (result *v1.OpenShiftControllerConfig, err error) {
	result = &v1.OpenShiftControllerConfig{}
	err = c.client.Put().
		Resource("openshiftcontrollerconfigs").
		Name(openShiftControllerConfig.Name).
		Body(openShiftControllerConfig).
		Do().
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().

func (c *openShiftControllerConfigs) UpdateStatus(openShiftControllerConfig *v1.OpenShiftControllerConfig) (result *v1.OpenShiftControllerConfig, err error) {
	result = &v1.OpenShiftControllerConfig{}
	err = c.client.Put().
		Resource("openshiftcontrollerconfigs").
		Name(openShiftControllerConfig.Name).
		SubResource("status").
		Body(openShiftControllerConfig).
		Do().
		Into(result)
	return
}

// Delete takes name of the openShiftControllerConfig and deletes it. Returns an error if one occurs.
func (c *openShiftControllerConfigs) Delete(name string, options *meta_v1.DeleteOptions) error {
	return c.client.Delete().
		Resource("openshiftcontrollerconfigs").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *openShiftControllerConfigs) DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error {
	return c.client.Delete().
		Resource("openshiftcontrollerconfigs").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched openShiftControllerConfig.
func (c *openShiftControllerConfigs) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.OpenShiftControllerConfig, err error) {
	result = &v1.OpenShiftControllerConfig{}
	err = c.client.Patch(pt).
		Resource("openshiftcontrollerconfigs").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
