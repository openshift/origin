package v1

import (
	v1 "github.com/openshift/origin/pkg/cmd/openshift-operators/orchestration-operator/apis/orchestration/v1"
	scheme "github.com/openshift/origin/pkg/cmd/openshift-operators/orchestration-operator/generated/clientset/versioned/scheme"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// OpenShiftOrchestrationConfigsGetter has a method to return a OpenShiftOrchestrationConfigInterface.
// A group's client should implement this interface.
type OpenShiftOrchestrationConfigsGetter interface {
	OpenShiftOrchestrationConfigs() OpenShiftOrchestrationConfigInterface
}

// OpenShiftOrchestrationConfigInterface has methods to work with OpenShiftOrchestrationConfig resources.
type OpenShiftOrchestrationConfigInterface interface {
	Create(*v1.OpenShiftOrchestrationConfig) (*v1.OpenShiftOrchestrationConfig, error)
	Update(*v1.OpenShiftOrchestrationConfig) (*v1.OpenShiftOrchestrationConfig, error)
	UpdateStatus(*v1.OpenShiftOrchestrationConfig) (*v1.OpenShiftOrchestrationConfig, error)
	Delete(name string, options *meta_v1.DeleteOptions) error
	DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error
	Get(name string, options meta_v1.GetOptions) (*v1.OpenShiftOrchestrationConfig, error)
	List(opts meta_v1.ListOptions) (*v1.OpenShiftOrchestrationConfigList, error)
	Watch(opts meta_v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.OpenShiftOrchestrationConfig, err error)
	OpenShiftOrchestrationConfigExpansion
}

// openShiftOrchestrationConfigs implements OpenShiftOrchestrationConfigInterface
type openShiftOrchestrationConfigs struct {
	client rest.Interface
}

// newOpenShiftOrchestrationConfigs returns a OpenShiftOrchestrationConfigs
func newOpenShiftOrchestrationConfigs(c *OrchestrationV1Client) *openShiftOrchestrationConfigs {
	return &openShiftOrchestrationConfigs{
		client: c.RESTClient(),
	}
}

// Get takes name of the openShiftOrchestrationConfig, and returns the corresponding openShiftOrchestrationConfig object, and an error if there is any.
func (c *openShiftOrchestrationConfigs) Get(name string, options meta_v1.GetOptions) (result *v1.OpenShiftOrchestrationConfig, err error) {
	result = &v1.OpenShiftOrchestrationConfig{}
	err = c.client.Get().
		Resource("openshiftorchestrationconfigs").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of OpenShiftOrchestrationConfigs that match those selectors.
func (c *openShiftOrchestrationConfigs) List(opts meta_v1.ListOptions) (result *v1.OpenShiftOrchestrationConfigList, err error) {
	result = &v1.OpenShiftOrchestrationConfigList{}
	err = c.client.Get().
		Resource("openshiftorchestrationconfigs").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested openShiftOrchestrationConfigs.
func (c *openShiftOrchestrationConfigs) Watch(opts meta_v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Resource("openshiftorchestrationconfigs").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Create takes the representation of a openShiftOrchestrationConfig and creates it.  Returns the server's representation of the openShiftOrchestrationConfig, and an error, if there is any.
func (c *openShiftOrchestrationConfigs) Create(openShiftOrchestrationConfig *v1.OpenShiftOrchestrationConfig) (result *v1.OpenShiftOrchestrationConfig, err error) {
	result = &v1.OpenShiftOrchestrationConfig{}
	err = c.client.Post().
		Resource("openshiftorchestrationconfigs").
		Body(openShiftOrchestrationConfig).
		Do().
		Into(result)
	return
}

// Update takes the representation of a openShiftOrchestrationConfig and updates it. Returns the server's representation of the openShiftOrchestrationConfig, and an error, if there is any.
func (c *openShiftOrchestrationConfigs) Update(openShiftOrchestrationConfig *v1.OpenShiftOrchestrationConfig) (result *v1.OpenShiftOrchestrationConfig, err error) {
	result = &v1.OpenShiftOrchestrationConfig{}
	err = c.client.Put().
		Resource("openshiftorchestrationconfigs").
		Name(openShiftOrchestrationConfig.Name).
		Body(openShiftOrchestrationConfig).
		Do().
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().

func (c *openShiftOrchestrationConfigs) UpdateStatus(openShiftOrchestrationConfig *v1.OpenShiftOrchestrationConfig) (result *v1.OpenShiftOrchestrationConfig, err error) {
	result = &v1.OpenShiftOrchestrationConfig{}
	err = c.client.Put().
		Resource("openshiftorchestrationconfigs").
		Name(openShiftOrchestrationConfig.Name).
		SubResource("status").
		Body(openShiftOrchestrationConfig).
		Do().
		Into(result)
	return
}

// Delete takes name of the openShiftOrchestrationConfig and deletes it. Returns an error if one occurs.
func (c *openShiftOrchestrationConfigs) Delete(name string, options *meta_v1.DeleteOptions) error {
	return c.client.Delete().
		Resource("openshiftorchestrationconfigs").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *openShiftOrchestrationConfigs) DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error {
	return c.client.Delete().
		Resource("openshiftorchestrationconfigs").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched openShiftOrchestrationConfig.
func (c *openShiftOrchestrationConfigs) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.OpenShiftOrchestrationConfig, err error) {
	result = &v1.OpenShiftOrchestrationConfig{}
	err = c.client.Patch(pt).
		Resource("openshiftorchestrationconfigs").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
