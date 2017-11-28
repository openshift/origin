package v1

import (
	v1 "github.com/openshift/api/security/v1"
	rest "k8s.io/client-go/rest"
)

// PodSecurityPolicySubjectReviewsGetter has a method to return a PodSecurityPolicySubjectReviewInterface.
// A group's client should implement this interface.
type PodSecurityPolicySubjectReviewsGetter interface {
	PodSecurityPolicySubjectReviews(namespace string) PodSecurityPolicySubjectReviewInterface
}

// PodSecurityPolicySubjectReviewInterface has methods to work with PodSecurityPolicySubjectReview resources.
type PodSecurityPolicySubjectReviewInterface interface {
	Create(*v1.PodSecurityPolicySubjectReview) (*v1.PodSecurityPolicySubjectReview, error)
	PodSecurityPolicySubjectReviewExpansion
}

// podSecurityPolicySubjectReviews implements PodSecurityPolicySubjectReviewInterface
type podSecurityPolicySubjectReviews struct {
	client rest.Interface
	ns     string
}

// newPodSecurityPolicySubjectReviews returns a PodSecurityPolicySubjectReviews
func newPodSecurityPolicySubjectReviews(c *SecurityV1Client, namespace string) *podSecurityPolicySubjectReviews {
	return &podSecurityPolicySubjectReviews{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Create takes the representation of a podSecurityPolicySubjectReview and creates it.  Returns the server's representation of the podSecurityPolicySubjectReview, and an error, if there is any.
func (c *podSecurityPolicySubjectReviews) Create(podSecurityPolicySubjectReview *v1.PodSecurityPolicySubjectReview) (result *v1.PodSecurityPolicySubjectReview, err error) {
	result = &v1.PodSecurityPolicySubjectReview{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("podsecuritypolicysubjectreviews").
		Body(podSecurityPolicySubjectReview).
		Do().
		Into(result)
	return
}
