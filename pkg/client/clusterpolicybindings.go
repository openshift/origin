package client

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

type ClusterPolicyBindingsInterface interface {
	ClusterPolicyBindings() ClusterPolicyBindingInterface
}

type ClusterPolicyBindingInterface interface {
	List(label labels.Selector, field fields.Selector) (*authorizationapi.ClusterPolicyBindingList, error)
	Get(name string) (*authorizationapi.ClusterPolicyBinding, error)
	Create(policyBinding *authorizationapi.ClusterPolicyBinding) (*authorizationapi.ClusterPolicyBinding, error)
	Delete(name string) error
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
func (c *clusterPolicyBindings) List(label labels.Selector, field fields.Selector) (result *authorizationapi.ClusterPolicyBindingList, err error) {
	result = &authorizationapi.ClusterPolicyBindingList{}
	err = c.r.Get().Resource("clusterPolicyBindings").LabelsSelectorParam(label).FieldsSelectorParam(field).Do().Into(result)
	return
}

// Get returns information about a particular policyBinding and error if one occurs.
func (c *clusterPolicyBindings) Get(name string) (result *authorizationapi.ClusterPolicyBinding, err error) {
	result = &authorizationapi.ClusterPolicyBinding{}
	err = c.r.Get().Resource("clusterPolicyBindings").Name(name).Do().Into(result)
	return
}

// Create creates new policyBinding. Returns the server's representation of the policyBinding and error if one occurs.
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
