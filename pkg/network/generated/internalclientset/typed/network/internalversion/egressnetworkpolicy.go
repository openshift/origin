package internalversion

import (
	network "github.com/openshift/origin/pkg/network/apis/network"
	scheme "github.com/openshift/origin/pkg/network/generated/internalclientset/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// EgressNetworkPoliciesGetter has a method to return a EgressNetworkPolicyInterface.
// A group's client should implement this interface.
type EgressNetworkPoliciesGetter interface {
	EgressNetworkPolicies(namespace string) EgressNetworkPolicyInterface
}

// EgressNetworkPolicyInterface has methods to work with EgressNetworkPolicy resources.
type EgressNetworkPolicyInterface interface {
	Create(*network.EgressNetworkPolicy) (*network.EgressNetworkPolicy, error)
	Update(*network.EgressNetworkPolicy) (*network.EgressNetworkPolicy, error)
	Delete(name string, options *v1.DeleteOptions) error
	DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error
	Get(name string, options v1.GetOptions) (*network.EgressNetworkPolicy, error)
	List(opts v1.ListOptions) (*network.EgressNetworkPolicyList, error)
	Watch(opts v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *network.EgressNetworkPolicy, err error)
	EgressNetworkPolicyExpansion
}

// egressNetworkPolicies implements EgressNetworkPolicyInterface
type egressNetworkPolicies struct {
	client rest.Interface
	ns     string
}

// newEgressNetworkPolicies returns a EgressNetworkPolicies
func newEgressNetworkPolicies(c *NetworkClient, namespace string) *egressNetworkPolicies {
	return &egressNetworkPolicies{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the egressNetworkPolicy, and returns the corresponding egressNetworkPolicy object, and an error if there is any.
func (c *egressNetworkPolicies) Get(name string, options v1.GetOptions) (result *network.EgressNetworkPolicy, err error) {
	result = &network.EgressNetworkPolicy{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("egressnetworkpolicies").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of EgressNetworkPolicies that match those selectors.
func (c *egressNetworkPolicies) List(opts v1.ListOptions) (result *network.EgressNetworkPolicyList, err error) {
	result = &network.EgressNetworkPolicyList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("egressnetworkpolicies").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested egressNetworkPolicies.
func (c *egressNetworkPolicies) Watch(opts v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("egressnetworkpolicies").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Create takes the representation of a egressNetworkPolicy and creates it.  Returns the server's representation of the egressNetworkPolicy, and an error, if there is any.
func (c *egressNetworkPolicies) Create(egressNetworkPolicy *network.EgressNetworkPolicy) (result *network.EgressNetworkPolicy, err error) {
	result = &network.EgressNetworkPolicy{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("egressnetworkpolicies").
		Body(egressNetworkPolicy).
		Do().
		Into(result)
	return
}

// Update takes the representation of a egressNetworkPolicy and updates it. Returns the server's representation of the egressNetworkPolicy, and an error, if there is any.
func (c *egressNetworkPolicies) Update(egressNetworkPolicy *network.EgressNetworkPolicy) (result *network.EgressNetworkPolicy, err error) {
	result = &network.EgressNetworkPolicy{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("egressnetworkpolicies").
		Name(egressNetworkPolicy.Name).
		Body(egressNetworkPolicy).
		Do().
		Into(result)
	return
}

// Delete takes name of the egressNetworkPolicy and deletes it. Returns an error if one occurs.
func (c *egressNetworkPolicies) Delete(name string, options *v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("egressnetworkpolicies").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *egressNetworkPolicies) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("egressnetworkpolicies").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched egressNetworkPolicy.
func (c *egressNetworkPolicies) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *network.EgressNetworkPolicy, err error) {
	result = &network.EgressNetworkPolicy{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("egressnetworkpolicies").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
