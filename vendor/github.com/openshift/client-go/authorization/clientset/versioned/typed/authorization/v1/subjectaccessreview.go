package v1

import (
	v1 "github.com/openshift/api/authorization/v1"
	rest "k8s.io/client-go/rest"
)

// SubjectAccessReviewsGetter has a method to return a SubjectAccessReviewInterface.
// A group's client should implement this interface.
type SubjectAccessReviewsGetter interface {
	SubjectAccessReviews() SubjectAccessReviewInterface
}

// SubjectAccessReviewInterface has methods to work with SubjectAccessReview resources.
type SubjectAccessReviewInterface interface {
	Create(*v1.SubjectAccessReview) (*v1.SubjectAccessReviewResponse, error)

	SubjectAccessReviewExpansion
}

// subjectAccessReviews implements SubjectAccessReviewInterface
type subjectAccessReviews struct {
	client rest.Interface
}

// newSubjectAccessReviews returns a SubjectAccessReviews
func newSubjectAccessReviews(c *AuthorizationV1Client) *subjectAccessReviews {
	return &subjectAccessReviews{
		client: c.RESTClient(),
	}
}

// Create takes the representation of a subjectAccessReview and creates it.  Returns the server's representation of the subjectAccessReviewResponse, and an error, if there is any.
func (c *subjectAccessReviews) Create(subjectAccessReview *v1.SubjectAccessReview) (result *v1.SubjectAccessReviewResponse, err error) {
	result = &v1.SubjectAccessReviewResponse{}
	err = c.client.Post().
		Resource("subjectaccessreviews").
		Body(subjectAccessReview).
		Do().
		Into(result)
	return
}
