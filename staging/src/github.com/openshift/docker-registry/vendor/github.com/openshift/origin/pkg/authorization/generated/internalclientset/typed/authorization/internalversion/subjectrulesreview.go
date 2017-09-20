package internalversion

import (
	authorization "github.com/openshift/origin/pkg/authorization/apis/authorization"
	rest "k8s.io/client-go/rest"
)

// SubjectRulesReviewsGetter has a method to return a SubjectRulesReviewInterface.
// A group's client should implement this interface.
type SubjectRulesReviewsGetter interface {
	SubjectRulesReviews(namespace string) SubjectRulesReviewInterface
}

// SubjectRulesReviewInterface has methods to work with SubjectRulesReview resources.
type SubjectRulesReviewInterface interface {
	Create(*authorization.SubjectRulesReview) (*authorization.SubjectRulesReview, error)
	SubjectRulesReviewExpansion
}

// subjectRulesReviews implements SubjectRulesReviewInterface
type subjectRulesReviews struct {
	client rest.Interface
	ns     string
}

// newSubjectRulesReviews returns a SubjectRulesReviews
func newSubjectRulesReviews(c *AuthorizationClient, namespace string) *subjectRulesReviews {
	return &subjectRulesReviews{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Create takes the representation of a subjectRulesReview and creates it.  Returns the server's representation of the subjectRulesReview, and an error, if there is any.
func (c *subjectRulesReviews) Create(subjectRulesReview *authorization.SubjectRulesReview) (result *authorization.SubjectRulesReview, err error) {
	result = &authorization.SubjectRulesReview{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("subjectrulesreviews").
		Body(subjectRulesReview).
		Do().
		Into(result)
	return
}
