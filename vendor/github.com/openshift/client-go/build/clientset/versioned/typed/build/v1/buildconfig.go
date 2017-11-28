package v1

import (
	v1 "github.com/openshift/api/build/v1"
	scheme "github.com/openshift/client-go/build/clientset/versioned/scheme"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// BuildConfigsGetter has a method to return a BuildConfigInterface.
// A group's client should implement this interface.
type BuildConfigsGetter interface {
	BuildConfigs(namespace string) BuildConfigInterface
}

// BuildConfigInterface has methods to work with BuildConfig resources.
type BuildConfigInterface interface {
	Create(*v1.BuildConfig) (*v1.BuildConfig, error)
	Update(*v1.BuildConfig) (*v1.BuildConfig, error)
	UpdateStatus(*v1.BuildConfig) (*v1.BuildConfig, error)
	Delete(name string, options *meta_v1.DeleteOptions) error
	DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error
	Get(name string, options meta_v1.GetOptions) (*v1.BuildConfig, error)
	List(opts meta_v1.ListOptions) (*v1.BuildConfigList, error)
	Watch(opts meta_v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.BuildConfig, err error)
	Instantiate(buildConfigName string, buildRequest *v1.BuildRequest) (*v1.Build, error)

	BuildConfigExpansion
}

// buildConfigs implements BuildConfigInterface
type buildConfigs struct {
	client rest.Interface
	ns     string
}

// newBuildConfigs returns a BuildConfigs
func newBuildConfigs(c *BuildV1Client, namespace string) *buildConfigs {
	return &buildConfigs{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the buildConfig, and returns the corresponding buildConfig object, and an error if there is any.
func (c *buildConfigs) Get(name string, options meta_v1.GetOptions) (result *v1.BuildConfig, err error) {
	result = &v1.BuildConfig{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("buildconfigs").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of BuildConfigs that match those selectors.
func (c *buildConfigs) List(opts meta_v1.ListOptions) (result *v1.BuildConfigList, err error) {
	result = &v1.BuildConfigList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("buildconfigs").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested buildConfigs.
func (c *buildConfigs) Watch(opts meta_v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("buildconfigs").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Create takes the representation of a buildConfig and creates it.  Returns the server's representation of the buildConfig, and an error, if there is any.
func (c *buildConfigs) Create(buildConfig *v1.BuildConfig) (result *v1.BuildConfig, err error) {
	result = &v1.BuildConfig{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("buildconfigs").
		Body(buildConfig).
		Do().
		Into(result)
	return
}

// Update takes the representation of a buildConfig and updates it. Returns the server's representation of the buildConfig, and an error, if there is any.
func (c *buildConfigs) Update(buildConfig *v1.BuildConfig) (result *v1.BuildConfig, err error) {
	result = &v1.BuildConfig{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("buildconfigs").
		Name(buildConfig.Name).
		Body(buildConfig).
		Do().
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().

func (c *buildConfigs) UpdateStatus(buildConfig *v1.BuildConfig) (result *v1.BuildConfig, err error) {
	result = &v1.BuildConfig{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("buildconfigs").
		Name(buildConfig.Name).
		SubResource("status").
		Body(buildConfig).
		Do().
		Into(result)
	return
}

// Delete takes name of the buildConfig and deletes it. Returns an error if one occurs.
func (c *buildConfigs) Delete(name string, options *meta_v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("buildconfigs").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *buildConfigs) DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("buildconfigs").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched buildConfig.
func (c *buildConfigs) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.BuildConfig, err error) {
	result = &v1.BuildConfig{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("buildconfigs").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}

// Instantiate takes the representation of a buildRequest and creates it.  Returns the server's representation of the build, and an error, if there is any.
func (c *buildConfigs) Instantiate(buildConfigName string, buildRequest *v1.BuildRequest) (result *v1.Build, err error) {
	result = &v1.Build{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("buildconfigs").
		Name(buildConfigName).
		SubResource("instantiate").
		Body(buildRequest).
		Do().
		Into(result)
	return
}
