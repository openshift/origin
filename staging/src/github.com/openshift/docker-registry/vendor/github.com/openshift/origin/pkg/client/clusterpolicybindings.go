package client

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	kapi "k8s.io/kubernetes/pkg/api"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
)

// ClusterPolicyBindingsInterface has methods to work with ClusterPolicyBindings resources in a namespace
type ClusterPolicyBindingsInterface interface {
	ClusterPolicyBindings() ClusterPolicyBindingInterface
}

// ClusterPolicyBindingInterface exposes methods on ClusterPolicyBindings resources
type ClusterPolicyBindingInterface interface {
	List(opts metav1.ListOptions) (*authorizationapi.ClusterPolicyBindingList, error)
	Get(name string, options metav1.GetOptions) (*authorizationapi.ClusterPolicyBinding, error)
	Create(policyBinding *authorizationapi.ClusterPolicyBinding) (*authorizationapi.ClusterPolicyBinding, error)
	Delete(name string) error
	Watch(opts metav1.ListOptions) (watch.Interface, error)
}

type ClusterPolicyBindingsListerInterface interface {
	ClusterPolicyBindings() ClusterPolicyBindingLister
}
type ClusterPolicyBindingLister interface {
	List(options metav1.ListOptions) (*authorizationapi.ClusterPolicyBindingList, error)
	Get(name string, options metav1.GetOptions) (*authorizationapi.ClusterPolicyBinding, error)
}
type SyncedClusterPolicyBindingsListerInterface interface {
	ClusterPolicyBindingsListerInterface
	LastSyncResourceVersion() string
}

type clusterPolicyBindings struct {
	r *Client
}

// newClusterPolicyBindings returns a clusterPolicyBindings
func newClusterPolicyBindings(c *Client) *clusterPolicyBindings {
	return &clusterPolicyBindings{
		r: c,
	}
}

// List returns a list of clusterPolicyBindings that match the label and field selectors.
func (c *clusterPolicyBindings) List(opts metav1.ListOptions) (result *authorizationapi.ClusterPolicyBindingList, err error) {
	result = &authorizationapi.ClusterPolicyBindingList{}
	err = c.r.Get().Resource("clusterPolicyBindings").VersionedParams(&opts, kapi.ParameterCodec).Do().Into(result)
	return
}

// Get returns information about a particular clusterPolicyBindings and error if one occurs.
func (c *clusterPolicyBindings) Get(name string, options metav1.GetOptions) (result *authorizationapi.ClusterPolicyBinding, err error) {
	result = &authorizationapi.ClusterPolicyBinding{}
	err = c.r.Get().Resource("clusterPolicyBindings").Name(name).VersionedParams(&options, kapi.ParameterCodec).Do().Into(result)
	return
}

// Create creates new policyBinding. Returns the server's representation of the clusterPolicyBindings and error if one occurs.
func (c *clusterPolicyBindings) Create(policyBinding *authorizationapi.ClusterPolicyBinding) (result *authorizationapi.ClusterPolicyBinding, err error) {
	result = &authorizationapi.ClusterPolicyBinding{}
	err = c.r.Post().Resource("clusterPolicyBindings").Body(policyBinding).Do().Into(result)
	return
}

// Delete deletes a policyBinding, returns error if one occurs.
func (c *clusterPolicyBindings) Delete(name string) (err error) {
	err = c.r.Delete().Resource("clusterPolicyBindings").Name(name).Do().Error()
	return
}

// Watch returns a watch.Interface that watches the requested clusterPolicyBindings
func (c *clusterPolicyBindings) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return c.r.Get().Prefix("watch").Resource("clusterPolicyBindings").VersionedParams(&opts, kapi.ParameterCodec).Watch()
}
