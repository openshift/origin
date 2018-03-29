package v1

import (
	v1 "github.com/openshift/origin/pkg/cmd/openshift-operators/apiserver-operator/apis/apiserver/v1"
	scheme "github.com/openshift/origin/pkg/cmd/openshift-operators/apiserver-operator/generated/clientset/versioned/scheme"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// OpenShiftAPIServerConfigsGetter has a method to return a OpenShiftAPIServerConfigInterface.
// A group's client should implement this interface.
type OpenShiftAPIServerConfigsGetter interface {
	OpenShiftAPIServerConfigs() OpenShiftAPIServerConfigInterface
}

// OpenShiftAPIServerConfigInterface has methods to work with OpenShiftAPIServerConfig resources.
type OpenShiftAPIServerConfigInterface interface {
	Create(*v1.OpenShiftAPIServerConfig) (*v1.OpenShiftAPIServerConfig, error)
	Update(*v1.OpenShiftAPIServerConfig) (*v1.OpenShiftAPIServerConfig, error)
	UpdateStatus(*v1.OpenShiftAPIServerConfig) (*v1.OpenShiftAPIServerConfig, error)
	Delete(name string, options *meta_v1.DeleteOptions) error
	DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error
	Get(name string, options meta_v1.GetOptions) (*v1.OpenShiftAPIServerConfig, error)
	List(opts meta_v1.ListOptions) (*v1.OpenShiftAPIServerConfigList, error)
	Watch(opts meta_v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.OpenShiftAPIServerConfig, err error)
	OpenShiftAPIServerConfigExpansion
}

// openShiftAPIServerConfigs implements OpenShiftAPIServerConfigInterface
type openShiftAPIServerConfigs struct {
	client rest.Interface
}

// newOpenShiftAPIServerConfigs returns a OpenShiftAPIServerConfigs
func newOpenShiftAPIServerConfigs(c *ApiserverV1Client) *openShiftAPIServerConfigs {
	return &openShiftAPIServerConfigs{
		client: c.RESTClient(),
	}
}

// Get takes name of the openShiftAPIServerConfig, and returns the corresponding openShiftAPIServerConfig object, and an error if there is any.
func (c *openShiftAPIServerConfigs) Get(name string, options meta_v1.GetOptions) (result *v1.OpenShiftAPIServerConfig, err error) {
	result = &v1.OpenShiftAPIServerConfig{}
	err = c.client.Get().
		Resource("openshiftapiserverconfigs").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of OpenShiftAPIServerConfigs that match those selectors.
func (c *openShiftAPIServerConfigs) List(opts meta_v1.ListOptions) (result *v1.OpenShiftAPIServerConfigList, err error) {
	result = &v1.OpenShiftAPIServerConfigList{}
	err = c.client.Get().
		Resource("openshiftapiserverconfigs").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested openShiftAPIServerConfigs.
func (c *openShiftAPIServerConfigs) Watch(opts meta_v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Resource("openshiftapiserverconfigs").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Create takes the representation of a openShiftAPIServerConfig and creates it.  Returns the server's representation of the openShiftAPIServerConfig, and an error, if there is any.
func (c *openShiftAPIServerConfigs) Create(openShiftAPIServerConfig *v1.OpenShiftAPIServerConfig) (result *v1.OpenShiftAPIServerConfig, err error) {
	result = &v1.OpenShiftAPIServerConfig{}
	err = c.client.Post().
		Resource("openshiftapiserverconfigs").
		Body(openShiftAPIServerConfig).
		Do().
		Into(result)
	return
}

// Update takes the representation of a openShiftAPIServerConfig and updates it. Returns the server's representation of the openShiftAPIServerConfig, and an error, if there is any.
func (c *openShiftAPIServerConfigs) Update(openShiftAPIServerConfig *v1.OpenShiftAPIServerConfig) (result *v1.OpenShiftAPIServerConfig, err error) {
	result = &v1.OpenShiftAPIServerConfig{}
	err = c.client.Put().
		Resource("openshiftapiserverconfigs").
		Name(openShiftAPIServerConfig.Name).
		Body(openShiftAPIServerConfig).
		Do().
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().

func (c *openShiftAPIServerConfigs) UpdateStatus(openShiftAPIServerConfig *v1.OpenShiftAPIServerConfig) (result *v1.OpenShiftAPIServerConfig, err error) {
	result = &v1.OpenShiftAPIServerConfig{}
	err = c.client.Put().
		Resource("openshiftapiserverconfigs").
		Name(openShiftAPIServerConfig.Name).
		SubResource("status").
		Body(openShiftAPIServerConfig).
		Do().
		Into(result)
	return
}

// Delete takes name of the openShiftAPIServerConfig and deletes it. Returns an error if one occurs.
func (c *openShiftAPIServerConfigs) Delete(name string, options *meta_v1.DeleteOptions) error {
	return c.client.Delete().
		Resource("openshiftapiserverconfigs").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *openShiftAPIServerConfigs) DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error {
	return c.client.Delete().
		Resource("openshiftapiserverconfigs").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched openShiftAPIServerConfig.
func (c *openShiftAPIServerConfigs) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.OpenShiftAPIServerConfig, err error) {
	result = &v1.OpenShiftAPIServerConfig{}
	err = c.client.Patch(pt).
		Resource("openshiftapiserverconfigs").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
