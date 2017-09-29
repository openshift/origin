package internalversion

import (
	authorization "github.com/openshift/origin/pkg/authorization/apis/authorization"
	rest "k8s.io/client-go/rest"
)

// LocalSubjectAccessReviewsGetter has a method to return a LocalSubjectAccessReviewInterface.
// A group's client should implement this interface.
type LocalSubjectAccessReviewsGetter interface {
	LocalSubjectAccessReviews(namespace string) LocalSubjectAccessReviewInterface
}

// LocalSubjectAccessReviewInterface has methods to work with LocalSubjectAccessReview resources.
type LocalSubjectAccessReviewInterface interface {
	Create(*authorization.LocalSubjectAccessReview) (*authorization.SubjectAccessReviewResponse, error)

	LocalSubjectAccessReviewExpansion
}

// localSubjectAccessReviews implements LocalSubjectAccessReviewInterface
type localSubjectAccessReviews struct {
	client rest.Interface
	ns     string
}

// newLocalSubjectAccessReviews returns a LocalSubjectAccessReviews
func newLocalSubjectAccessReviews(c *AuthorizationClient, namespace string) *localSubjectAccessReviews {
	return &localSubjectAccessReviews{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Create takes the representation of a localSubjectAccessReview and creates it.  Returns the server's representation of the subjectAccessReviewResponse, and an error, if there is any.
func (c *localSubjectAccessReviews) Create(localSubjectAccessReview *authorization.LocalSubjectAccessReview) (result *authorization.SubjectAccessReviewResponse, err error) {
	result = &authorization.SubjectAccessReviewResponse{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("localsubjectaccessreviews").
		Body(localSubjectAccessReview).
		Do().
		Into(result)
	return
}
