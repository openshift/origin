package client

import (
	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

// ResourceAccessReviewsNamespacer has methods to work with ResourceAccessReview resources in a namespace
type ResourceAccessReviewsNamespacer interface {
	ResourceAccessReviews(namespace string) ResourceAccessReviewInterface
}

// ClusterResourceAccessReviews has methods to work with ResourceAccessReview resources in the cluster scope
type ClusterResourceAccessReviews interface {
	ClusterResourceAccessReviews() ResourceAccessReviewInterface
}

// ResourceAccessReviewInterface exposes methods on ResourceAccessReview resources.
type ResourceAccessReviewInterface interface {
	Create(policy *authorizationapi.ResourceAccessReview) (*authorizationapi.ResourceAccessReviewResponse, error)
}

// resourceAccessReviews implements ResourceAccessReviewsNamespacer interface
type resourceAccessReviews struct {
	r  *Client
	ns string
}

// newResourceAccessReviews returns a resourceAccessReviews
func newResourceAccessReviews(c *Client, namespace string) *resourceAccessReviews {
	return &resourceAccessReviews{
		r:  c,
		ns: namespace,
	}
}

// Create creates new policy. Returns the server's representation of the policy and error if one occurs.
func (c *resourceAccessReviews) Create(policy *authorizationapi.ResourceAccessReview) (result *authorizationapi.ResourceAccessReviewResponse, err error) {
	result = &authorizationapi.ResourceAccessReviewResponse{}
	err = c.r.Post().Namespace(c.ns).Resource("resourceAccessReviews").Body(policy).Do().Into(result)
	return
}

// clusterResourceAccessReviews implements ClusterResourceAccessReviews interface
type clusterResourceAccessReviews struct {
	r *Client
}

// newClusterResourceAccessReviews returns a clusterResourceAccessReviews
func newClusterResourceAccessReviews(c *Client) *clusterResourceAccessReviews {
	return &clusterResourceAccessReviews{
		r: c,
	}
}

// Create creates new policy. Returns the server's representation of the policy and error if one occurs.
func (c *clusterResourceAccessReviews) Create(policy *authorizationapi.ResourceAccessReview) (result *authorizationapi.ResourceAccessReviewResponse, err error) {
	result = &authorizationapi.ResourceAccessReviewResponse{}
	err = c.r.Post().Resource("resourceAccessReviews").Body(policy).Do().Into(result)
	return
}
