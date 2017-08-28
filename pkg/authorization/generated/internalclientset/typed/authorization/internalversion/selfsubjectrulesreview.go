package internalversion

import (
	authorization "github.com/openshift/origin/pkg/authorization/apis/authorization"
	rest "k8s.io/client-go/rest"
)

// SelfSubjectRulesReviewsGetter has a method to return a SelfSubjectRulesReviewInterface.
// A group's client should implement this interface.
type SelfSubjectRulesReviewsGetter interface {
	SelfSubjectRulesReviews(namespace string) SelfSubjectRulesReviewInterface
}

// SelfSubjectRulesReviewInterface has methods to work with SelfSubjectRulesReview resources.
type SelfSubjectRulesReviewInterface interface {
	Create(*authorization.SelfSubjectRulesReview) (*authorization.SelfSubjectRulesReview, error)
	SelfSubjectRulesReviewExpansion
}

// selfSubjectRulesReviews implements SelfSubjectRulesReviewInterface
type selfSubjectRulesReviews struct {
	client rest.Interface
	ns     string
}

// newSelfSubjectRulesReviews returns a SelfSubjectRulesReviews
func newSelfSubjectRulesReviews(c *AuthorizationClient, namespace string) *selfSubjectRulesReviews {
	return &selfSubjectRulesReviews{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Create takes the representation of a selfSubjectRulesReview and creates it.  Returns the server's representation of the selfSubjectRulesReview, and an error, if there is any.
func (c *selfSubjectRulesReviews) Create(selfSubjectRulesReview *authorization.SelfSubjectRulesReview) (result *authorization.SelfSubjectRulesReview, err error) {
	result = &authorization.SelfSubjectRulesReview{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("selfsubjectrulesreviews").
		Body(selfSubjectRulesReview).
		Do().
		Into(result)
	return
}
