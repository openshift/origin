package client

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	kapi "k8s.io/kubernetes/pkg/api"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
)

// PoliciesNamespacer has methods to work with Policy resources in a namespace
type PoliciesNamespacer interface {
	Policies(namespace string) PolicyInterface
}

// PolicyInterface exposes methods on Policy resources.
type PolicyInterface interface {
	List(opts metav1.ListOptions) (*authorizationapi.PolicyList, error)
	Get(name string, options metav1.GetOptions) (*authorizationapi.Policy, error)
	Delete(name string) error
	Watch(opts metav1.ListOptions) (watch.Interface, error)
}

type PoliciesListerNamespacer interface {
	Policies(namespace string) PolicyLister
}
type SyncedPoliciesListerNamespacer interface {
	PoliciesListerNamespacer
	LastSyncResourceVersion() string
}
type PolicyLister interface {
	List(options metav1.ListOptions) (*authorizationapi.PolicyList, error)
	Get(name string, options metav1.GetOptions) (*authorizationapi.Policy, error)
}

// policies implements PoliciesNamespacer interface
type policies struct {
	r  *Client
	ns string
}

// newPolicies returns a policies
func newPolicies(c *Client, namespace string) *policies {
	return &policies{
		r:  c,
		ns: namespace,
	}
}

// List returns a list of policies that match the label and field selectors.
func (c *policies) List(opts metav1.ListOptions) (result *authorizationapi.PolicyList, err error) {
	result = &authorizationapi.PolicyList{}
	err = c.r.Get().Namespace(c.ns).Resource("policies").VersionedParams(&opts, kapi.ParameterCodec).Do().Into(result)
	return
}

// Get returns information about a particular policy and error if one occurs.
func (c *policies) Get(name string, options metav1.GetOptions) (result *authorizationapi.Policy, err error) {
	result = &authorizationapi.Policy{}
	err = c.r.Get().Namespace(c.ns).Resource("policies").Name(name).VersionedParams(&options, kapi.ParameterCodec).Do().Into(result)
	return
}

// Delete deletes a policy, returns error if one occurs.
func (c *policies) Delete(name string) (err error) {
	err = c.r.Delete().Namespace(c.ns).Resource("policies").Name(name).Do().Error()
	return
}

// Watch returns a watch.Interface that watches the requested policies
func (c *policies) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return c.r.Get().Prefix("watch").Namespace(c.ns).Resource("policies").VersionedParams(&opts, kapi.ParameterCodec).Watch()
}
