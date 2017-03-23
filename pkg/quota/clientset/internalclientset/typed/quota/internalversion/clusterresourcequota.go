package internalversion

import (
	api "github.com/openshift/origin/pkg/quota/api"
	pkg_api "k8s.io/kubernetes/pkg/api"
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
	Create(*api.ClusterResourceQuota) (*api.ClusterResourceQuota, error)
	Update(*api.ClusterResourceQuota) (*api.ClusterResourceQuota, error)
	Delete(name string, options *pkg_api.DeleteOptions) error
	DeleteCollection(options *pkg_api.DeleteOptions, listOptions pkg_api.ListOptions) error
	Get(name string) (*api.ClusterResourceQuota, error)
	List(opts pkg_api.ListOptions) (*api.ClusterResourceQuotaList, error)
	Watch(opts pkg_api.ListOptions) (watch.Interface, error)
	Patch(name string, pt pkg_api.PatchType, data []byte, subresources ...string) (result *api.ClusterResourceQuota, err error)
	ClusterResourceQuotaExpansion
}

// clusterResourceQuotas implements ClusterResourceQuotaInterface
type clusterResourceQuotas struct {
	client restclient.Interface
}

// newClusterResourceQuotas returns a ClusterResourceQuotas
func newClusterResourceQuotas(c *QuotaClient) *clusterResourceQuotas {
	return &clusterResourceQuotas{
		client: c.RESTClient(),
	}
}

// Create takes the representation of a clusterResourceQuota and creates it.  Returns the server's representation of the clusterResourceQuota, and an error, if there is any.
func (c *clusterResourceQuotas) Create(clusterResourceQuota *api.ClusterResourceQuota) (result *api.ClusterResourceQuota, err error) {
	result = &api.ClusterResourceQuota{}
	err = c.client.Post().
		Resource("clusterresourcequotas").
		Body(clusterResourceQuota).
		Do().
		Into(result)
	return
}

// Update takes the representation of a clusterResourceQuota and updates it. Returns the server's representation of the clusterResourceQuota, and an error, if there is any.
func (c *clusterResourceQuotas) Update(clusterResourceQuota *api.ClusterResourceQuota) (result *api.ClusterResourceQuota, err error) {
	result = &api.ClusterResourceQuota{}
	err = c.client.Put().
		Resource("clusterresourcequotas").
		Name(clusterResourceQuota.Name).
		Body(clusterResourceQuota).
		Do().
		Into(result)
	return
}

// Delete takes name of the clusterResourceQuota and deletes it. Returns an error if one occurs.
func (c *clusterResourceQuotas) Delete(name string, options *pkg_api.DeleteOptions) error {
	return c.client.Delete().
		Resource("clusterresourcequotas").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *clusterResourceQuotas) DeleteCollection(options *pkg_api.DeleteOptions, listOptions pkg_api.ListOptions) error {
	return c.client.Delete().
		Resource("clusterresourcequotas").
		VersionedParams(&listOptions, pkg_api.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Get takes name of the clusterResourceQuota, and returns the corresponding clusterResourceQuota object, and an error if there is any.
func (c *clusterResourceQuotas) Get(name string) (result *api.ClusterResourceQuota, err error) {
	result = &api.ClusterResourceQuota{}
	err = c.client.Get().
		Resource("clusterresourcequotas").
		Name(name).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of ClusterResourceQuotas that match those selectors.
func (c *clusterResourceQuotas) List(opts pkg_api.ListOptions) (result *api.ClusterResourceQuotaList, err error) {
	result = &api.ClusterResourceQuotaList{}
	err = c.client.Get().
		Resource("clusterresourcequotas").
		VersionedParams(&opts, pkg_api.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested clusterResourceQuotas.
func (c *clusterResourceQuotas) Watch(opts pkg_api.ListOptions) (watch.Interface, error) {
	return c.client.Get().
		Prefix("watch").
		Resource("clusterresourcequotas").
		VersionedParams(&opts, pkg_api.ParameterCodec).
		Watch()
}

// Patch applies the patch and returns the patched clusterResourceQuota.
func (c *clusterResourceQuotas) Patch(name string, pt pkg_api.PatchType, data []byte, subresources ...string) (result *api.ClusterResourceQuota, err error) {
	result = &api.ClusterResourceQuota{}
	err = c.client.Patch(pt).
		Resource("clusterresourcequotas").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
