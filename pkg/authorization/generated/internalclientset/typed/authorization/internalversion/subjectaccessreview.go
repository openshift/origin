package internalversion

import (
	authorization "github.com/openshift/origin/pkg/authorization/apis/authorization"
	rest "k8s.io/client-go/rest"
)

// SubjectAccessReviewsGetter has a method to return a SubjectAccessReviewInterface.
// A group's client should implement this interface.
type SubjectAccessReviewsGetter interface {
	SubjectAccessReviews() SubjectAccessReviewInterface
}

// SubjectAccessReviewInterface has methods to work with SubjectAccessReview resources.
type SubjectAccessReviewInterface interface {
	Create(*authorization.SubjectAccessReview) (*authorization.SubjectAccessReviewResponse, error)

	SubjectAccessReviewExpansion
}

// subjectAccessReviews implements SubjectAccessReviewInterface
type subjectAccessReviews struct {
	client rest.Interface
}

// newSubjectAccessReviews returns a SubjectAccessReviews
func newSubjectAccessReviews(c *AuthorizationClient) *subjectAccessReviews {
	return &subjectAccessReviews{
		client: c.RESTClient(),
	}
}

// Create takes the representation of a subjectAccessReview and creates it.  Returns the server's representation of the subjectAccessReviewResponse, and an error, if there is any.
func (c *subjectAccessReviews) Create(subjectAccessReview *authorization.SubjectAccessReview) (result *authorization.SubjectAccessReviewResponse, err error) {
	result = &authorization.SubjectAccessReviewResponse{}
	err = c.client.Post().
		Resource("subjectaccessreviews").
		Body(subjectAccessReview).
		Do().
		Into(result)
	return
}
