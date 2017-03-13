package client

import (
	metainternal "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	"k8s.io/apimachinery/pkg/watch"
	kapi "k8s.io/kubernetes/pkg/api"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

// ClusterPoliciesInterface has methods to work with ClusterPolicies resources in a namespace
type ClusterPoliciesInterface interface {
	ClusterPolicies() ClusterPolicyInterface
}

// ClusterPolicyInterface exposes methods on ClusterPolicies resources
type ClusterPolicyInterface interface {
	List(opts metainternal.ListOptions) (*authorizationapi.ClusterPolicyList, error)
	Get(name string) (*authorizationapi.ClusterPolicy, error)
	Delete(name string) error
	Watch(opts metainternal.ListOptions) (watch.Interface, error)
}

type ClusterPoliciesListerInterface interface {
	ClusterPolicies() ClusterPolicyLister
}
type ClusterPolicyLister interface {
	List(options metainternal.ListOptions) (*authorizationapi.ClusterPolicyList, error)
	Get(name string) (*authorizationapi.ClusterPolicy, error)
}
type SyncedClusterPoliciesListerInterface interface {
	ClusterPoliciesListerInterface
	LastSyncResourceVersion() string
}

type clusterPolicies struct {
	r *Client
}

func newClusterPolicies(c *Client) *clusterPolicies {
	return &clusterPolicies{
		r: c,
	}
}

// List returns a list of policies that match the label and field selectors.
func (c *clusterPolicies) List(opts metainternal.ListOptions) (result *authorizationapi.ClusterPolicyList, err error) {
	result = &authorizationapi.ClusterPolicyList{}
	err = c.r.Get().Resource("clusterPolicies").VersionedParams(&opts, kapi.ParameterCodec).Do().Into(result)
	return
}

// Get returns information about a particular policy and error if one occurs.
func (c *clusterPolicies) Get(name string) (result *authorizationapi.ClusterPolicy, err error) {
	result = &authorizationapi.ClusterPolicy{}
	err = c.r.Get().Resource("clusterPolicies").Name(name).Do().Into(result)
	return
}

// Delete deletes a policy, returns error if one occurs.
func (c *clusterPolicies) Delete(name string) (err error) {
	err = c.r.Delete().Resource("clusterPolicies").Name(name).Do().Error()
	return
}

// Watch returns a watch.Interface that watches the requested clusterPolicies
func (c *clusterPolicies) Watch(opts metainternal.ListOptions) (watch.Interface, error) {
	return c.r.Get().Prefix("watch").Resource("clusterPolicies").VersionedParams(&opts, kapi.ParameterCodec).Watch()
}
