package internalversion

import (
	authorization "github.com/openshift/origin/pkg/authorization/apis/authorization"
	scheme "github.com/openshift/origin/pkg/authorization/generated/internalclientset/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// ClusterPoliciesGetter has a method to return a ClusterPolicyInterface.
// A group's client should implement this interface.
type ClusterPoliciesGetter interface {
	ClusterPolicies() ClusterPolicyInterface
}

// ClusterPolicyInterface has methods to work with ClusterPolicy resources.
type ClusterPolicyInterface interface {
	Create(*authorization.ClusterPolicy) (*authorization.ClusterPolicy, error)
	Update(*authorization.ClusterPolicy) (*authorization.ClusterPolicy, error)
	Delete(name string, options *v1.DeleteOptions) error
	DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error
	Get(name string, options v1.GetOptions) (*authorization.ClusterPolicy, error)
	List(opts v1.ListOptions) (*authorization.ClusterPolicyList, error)
	Watch(opts v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *authorization.ClusterPolicy, err error)
	ClusterPolicyExpansion
}

// clusterPolicies implements ClusterPolicyInterface
type clusterPolicies struct {
	client rest.Interface
}

// newClusterPolicies returns a ClusterPolicies
func newClusterPolicies(c *AuthorizationClient) *clusterPolicies {
	return &clusterPolicies{
		client: c.RESTClient(),
	}
}

// Get takes name of the clusterPolicy, and returns the corresponding clusterPolicy object, and an error if there is any.
func (c *clusterPolicies) Get(name string, options v1.GetOptions) (result *authorization.ClusterPolicy, err error) {
	result = &authorization.ClusterPolicy{}
	err = c.client.Get().
		Resource("clusterpolicies").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of ClusterPolicies that match those selectors.
func (c *clusterPolicies) List(opts v1.ListOptions) (result *authorization.ClusterPolicyList, err error) {
	result = &authorization.ClusterPolicyList{}
	err = c.client.Get().
		Resource("clusterpolicies").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested clusterPolicies.
func (c *clusterPolicies) Watch(opts v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Resource("clusterpolicies").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Create takes the representation of a clusterPolicy and creates it.  Returns the server's representation of the clusterPolicy, and an error, if there is any.
func (c *clusterPolicies) Create(clusterPolicy *authorization.ClusterPolicy) (result *authorization.ClusterPolicy, err error) {
	result = &authorization.ClusterPolicy{}
	err = c.client.Post().
		Resource("clusterpolicies").
		Body(clusterPolicy).
		Do().
		Into(result)
	return
}

// Update takes the representation of a clusterPolicy and updates it. Returns the server's representation of the clusterPolicy, and an error, if there is any.
func (c *clusterPolicies) Update(clusterPolicy *authorization.ClusterPolicy) (result *authorization.ClusterPolicy, err error) {
	result = &authorization.ClusterPolicy{}
	err = c.client.Put().
		Resource("clusterpolicies").
		Name(clusterPolicy.Name).
		Body(clusterPolicy).
		Do().
		Into(result)
	return
}

// Delete takes name of the clusterPolicy and deletes it. Returns an error if one occurs.
func (c *clusterPolicies) Delete(name string, options *v1.DeleteOptions) error {
	return c.client.Delete().
		Resource("clusterpolicies").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *clusterPolicies) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	return c.client.Delete().
		Resource("clusterpolicies").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched clusterPolicy.
func (c *clusterPolicies) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *authorization.ClusterPolicy, err error) {
	result = &authorization.ClusterPolicy{}
	err = c.client.Patch(pt).
		Resource("clusterpolicies").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
