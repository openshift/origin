package internalversion

import (
	authorization "github.com/openshift/origin/pkg/authorization/apis/authorization"
	rest "k8s.io/client-go/rest"
)

// ResourceAccessReviewsGetter has a method to return a ResourceAccessReviewInterface.
// A group's client should implement this interface.
type ResourceAccessReviewsGetter interface {
	ResourceAccessReviews() ResourceAccessReviewInterface
}

// ResourceAccessReviewInterface has methods to work with ResourceAccessReview resources.
type ResourceAccessReviewInterface interface {
	Create(*authorization.ResourceAccessReview) (*authorization.ResourceAccessReviewResponse, error)

	ResourceAccessReviewExpansion
}

// resourceAccessReviews implements ResourceAccessReviewInterface
type resourceAccessReviews struct {
	client rest.Interface
}

// newResourceAccessReviews returns a ResourceAccessReviews
func newResourceAccessReviews(c *AuthorizationClient) *resourceAccessReviews {
	return &resourceAccessReviews{
		client: c.RESTClient(),
	}
}

// Create takes the representation of a resourceAccessReview and creates it.  Returns the server's representation of the resourceAccessReviewResponse, and an error, if there is any.
func (c *resourceAccessReviews) Create(resourceAccessReview *authorization.ResourceAccessReview) (result *authorization.ResourceAccessReviewResponse, err error) {
	result = &authorization.ResourceAccessReviewResponse{}
	err = c.client.Post().
		Resource("resourceaccessreviews").
		Body(resourceAccessReview).
		Do().
		Into(result)
	return
}
