package client

import (
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/watch"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

// ClusterPoliciesInterface has methods to work with ClusterPolicies resources in a namespace
type ClusterPoliciesInterface interface {
	ClusterPolicies() ClusterPolicyInterface
}

// ClusterPolicyInterface exposes methods on ClusterPolicies resources
type ClusterPolicyInterface interface {
	List(label labels.Selector, field fields.Selector) (*authorizationapi.ClusterPolicyList, error)
	Get(name string) (*authorizationapi.ClusterPolicy, error)
	Delete(name string) error
	Watch(label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error)
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
func (c *clusterPolicies) List(label labels.Selector, field fields.Selector) (result *authorizationapi.ClusterPolicyList, err error) {
	result = &authorizationapi.ClusterPolicyList{}
	err = c.r.Get().Resource("clusterPolicies").LabelsSelectorParam(label).FieldsSelectorParam(field).Do().Into(result)
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
func (c *clusterPolicies) Watch(label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error) {
	return c.r.Get().Prefix("watch").Resource("clusterPolicies").Param("resourceVersion", resourceVersion).LabelsSelectorParam(label).FieldsSelectorParam(field).Watch()
}
