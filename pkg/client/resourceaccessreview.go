package client

import (
	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

// ResourceAccessReviewsNamespacer has methods to work with ResourceAccessReview resources in a namespace
type ResourceAccessReviewsNamespacer interface {
	ResourceAccessReviews(namespace string) ResourceAccessReviewInterface
}

// RootResourceAccessReviews has methods to work with ResourceAccessReview resources in the root scope
type RootResourceAccessReviews interface {
	RootResourceAccessReviews() ResourceAccessReviewInterface
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

// rootResourceAccessReviews implements RootResourceAccessReviews interface
type rootResourceAccessReviews struct {
	r *Client
}

// newRootResourceAccessReviews returns a rootResourceAccessReviews
func newRootResourceAccessReviews(c *Client) *rootResourceAccessReviews {
	return &rootResourceAccessReviews{
		r: c,
	}
}

// Create creates new policy. Returns the server's representation of the policy and error if one occurs.
func (c *rootResourceAccessReviews) Create(policy *authorizationapi.ResourceAccessReview) (result *authorizationapi.ResourceAccessReviewResponse, err error) {
	result = &authorizationapi.ResourceAccessReviewResponse{}
	err = c.r.Post().Resource("resourceAccessReviews").Body(policy).Do().Into(result)
	return
}
