package v1

import (
	v1 "github.com/openshift/origin/pkg/quota/api/v1"
	api "k8s.io/kubernetes/pkg/api"
	api_v1 "k8s.io/kubernetes/pkg/api/v1"
	restclient "k8s.io/kubernetes/pkg/client/restclient"
	watch "k8s.io/kubernetes/pkg/watch"
)

// ClusterResourceQuotasGetter has a method to return a ClusterResourceQuotaInterface.
// A group's client should implement this interface.
type ClusterResourceQuotasGetter interface {
	ClusterResourceQuotas() ClusterResourceQuotaInterface
}

// ClusterResourceQuotaInterface has methods to work with ClusterResourceQuota resources.
type ClusterResourceQuotaInterface interface {
	Create(*v1.ClusterResourceQuota) (*v1.ClusterResourceQuota, error)
	Update(*v1.ClusterResourceQuota) (*v1.ClusterResourceQuota, error)
	UpdateStatus(*v1.ClusterResourceQuota) (*v1.ClusterResourceQuota, error)
	Delete(name string, options *api_v1.DeleteOptions) error
	DeleteCollection(options *api_v1.DeleteOptions, listOptions api_v1.ListOptions) error
	Get(name string) (*v1.ClusterResourceQuota, error)
	List(opts api_v1.ListOptions) (*v1.ClusterResourceQuotaList, error)
	Watch(opts api_v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt api.PatchType, data []byte, subresources ...string) (result *v1.ClusterResourceQuota, err error)
	ClusterResourceQuotaExpansion
}

// clusterResourceQuotas implements ClusterResourceQuotaInterface
type clusterResourceQuotas struct {
	client restclient.Interface
}

// newClusterResourceQuotas returns a ClusterResourceQuotas
func newClusterResourceQuotas(c *QuotaV1Client) *clusterResourceQuotas {
	return &clusterResourceQuotas{
		client: c.RESTClient(),
	}
}

// Create takes the representation of a clusterResourceQuota and creates it.  Returns the server's representation of the clusterResourceQuota, and an error, if there is any.
func (c *clusterResourceQuotas) Create(clusterResourceQuota *v1.ClusterResourceQuota) (result *v1.ClusterResourceQuota, err error) {
	result = &v1.ClusterResourceQuota{}
	err = c.client.Post().
		Resource("clusterresourcequotas").
		Body(clusterResourceQuota).
		Do().
		Into(result)
	return
}

// Update takes the representation of a clusterResourceQuota and updates it. Returns the server's representation of the clusterResourceQuota, and an error, if there is any.
func (c *clusterResourceQuotas) Update(clusterResourceQuota *v1.ClusterResourceQuota) (result *v1.ClusterResourceQuota, err error) {
	result = &v1.ClusterResourceQuota{}
	err = c.client.Put().
		Resource("clusterresourcequotas").
		Name(clusterResourceQuota.Name).
		Body(clusterResourceQuota).
		Do().
		Into(result)
	return
}

func (c *clusterResourceQuotas) UpdateStatus(clusterResourceQuota *v1.ClusterResourceQuota) (result *v1.ClusterResourceQuota, err error) {
	result = &v1.ClusterResourceQuota{}
	err = c.client.Put().
		Resource("clusterresourcequotas").
		Name(clusterResourceQuota.Name).
		SubResource("status").
		Body(clusterResourceQuota).
		Do().
		Into(result)
	return
}

// Delete takes name of the clusterResourceQuota and deletes it. Returns an error if one occurs.
func (c *clusterResourceQuotas) Delete(name string, options *api_v1.DeleteOptions) error {
	return c.client.Delete().
		Resource("clusterresourcequotas").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *clusterResourceQuotas) DeleteCollection(options *api_v1.DeleteOptions, listOptions api_v1.ListOptions) error {
	return c.client.Delete().
		Resource("clusterresourcequotas").
		VersionedParams(&listOptions, api.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Get takes name of the clusterResourceQuota, and returns the corresponding clusterResourceQuota object, and an error if there is any.
func (c *clusterResourceQuotas) Get(name string) (result *v1.ClusterResourceQuota, err error) {
	result = &v1.ClusterResourceQuota{}
	err = c.client.Get().
		Resource("clusterresourcequotas").
		Name(name).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of ClusterResourceQuotas that match those selectors.
func (c *clusterResourceQuotas) List(opts api_v1.ListOptions) (result *v1.ClusterResourceQuotaList, err error) {
	result = &v1.ClusterResourceQuotaList{}
	err = c.client.Get().
		Resource("clusterresourcequotas").
		VersionedParams(&opts, api.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested clusterResourceQuotas.
func (c *clusterResourceQuotas) Watch(opts api_v1.ListOptions) (watch.Interface, error) {
	return c.client.Get().
		Prefix("watch").
		Resource("clusterresourcequotas").
		VersionedParams(&opts, api.ParameterCodec).
		Watch()
}

// Patch applies the patch and returns the patched clusterResourceQuota.
func (c *clusterResourceQuotas) Patch(name string, pt api.PatchType, data []byte, subresources ...string) (result *v1.ClusterResourceQuota, err error) {
	result = &v1.ClusterResourceQuota{}
	err = c.client.Patch(pt).
		Resource("clusterresourcequotas").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
