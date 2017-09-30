package internalversion

import (
	security "github.com/openshift/origin/pkg/security/apis/security"
	rest "k8s.io/client-go/rest"
)

// PodSecurityPolicySubjectReviewsGetter has a method to return a PodSecurityPolicySubjectReviewInterface.
// A group's client should implement this interface.
type PodSecurityPolicySubjectReviewsGetter interface {
	PodSecurityPolicySubjectReviews(namespace string) PodSecurityPolicySubjectReviewInterface
}

// PodSecurityPolicySubjectReviewInterface has methods to work with PodSecurityPolicySubjectReview resources.
type PodSecurityPolicySubjectReviewInterface interface {
	Create(*security.PodSecurityPolicySubjectReview) (*security.PodSecurityPolicySubjectReview, error)
	PodSecurityPolicySubjectReviewExpansion
}

// podSecurityPolicySubjectReviews implements PodSecurityPolicySubjectReviewInterface
type podSecurityPolicySubjectReviews struct {
	client rest.Interface
	ns     string
}

// newPodSecurityPolicySubjectReviews returns a PodSecurityPolicySubjectReviews
func newPodSecurityPolicySubjectReviews(c *SecurityClient, namespace string) *podSecurityPolicySubjectReviews {
	return &podSecurityPolicySubjectReviews{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Create takes the representation of a podSecurityPolicySubjectReview and creates it.  Returns the server's representation of the podSecurityPolicySubjectReview, and an error, if there is any.
func (c *podSecurityPolicySubjectReviews) Create(podSecurityPolicySubjectReview *security.PodSecurityPolicySubjectReview) (result *security.PodSecurityPolicySubjectReview, err error) {
	result = &security.PodSecurityPolicySubjectReview{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("podsecuritypolicysubjectreviews").
		Body(podSecurityPolicySubjectReview).
		Do().
		Into(result)
	return
}
