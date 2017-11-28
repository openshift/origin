package v1

import (
	v1 "github.com/openshift/api/authorization/v1"
	rest "k8s.io/client-go/rest"
)

// SelfSubjectRulesReviewsGetter has a method to return a SelfSubjectRulesReviewInterface.
// A group's client should implement this interface.
type SelfSubjectRulesReviewsGetter interface {
	SelfSubjectRulesReviews(namespace string) SelfSubjectRulesReviewInterface
}

// SelfSubjectRulesReviewInterface has methods to work with SelfSubjectRulesReview resources.
type SelfSubjectRulesReviewInterface interface {
	Create(*v1.SelfSubjectRulesReview) (*v1.SelfSubjectRulesReview, error)
	SelfSubjectRulesReviewExpansion
}

// selfSubjectRulesReviews implements SelfSubjectRulesReviewInterface
type selfSubjectRulesReviews struct {
	client rest.Interface
	ns     string
}

// newSelfSubjectRulesReviews returns a SelfSubjectRulesReviews
func newSelfSubjectRulesReviews(c *AuthorizationV1Client, namespace string) *selfSubjectRulesReviews {
	return &selfSubjectRulesReviews{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Create takes the representation of a selfSubjectRulesReview and creates it.  Returns the server's representation of the selfSubjectRulesReview, and an error, if there is any.
func (c *selfSubjectRulesReviews) Create(selfSubjectRulesReview *v1.SelfSubjectRulesReview) (result *v1.SelfSubjectRulesReview, err error) {
	result = &v1.SelfSubjectRulesReview{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("selfsubjectrulesreviews").
		Body(selfSubjectRulesReview).
		Do().
		Into(result)
	return
}
