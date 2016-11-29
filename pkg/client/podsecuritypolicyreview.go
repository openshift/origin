package client

import securityapi "github.com/openshift/origin/pkg/security/api"

// PodSecurityPolicyReviewsNamespacer has methods to work with PodSecurityPolicyReview resources in the cluster scope
type PodSecurityPolicyReviewsNamespacer interface {
	PodSecurityPolicyReviews(namespace string) PodSecurityPolicyReviewInterface
}

// PodSecurityPolicyReviewInterface exposes methods on PodSecurityPolicyReview resources.
type PodSecurityPolicyReviewInterface interface {
	Create(policy *securityapi.PodSecurityPolicyReview) (*securityapi.PodSecurityPolicyReview, error)
}

// podSecurityPolicyReviews implements PodSecurityPolicyReviewsNamespacer interface
type podSecurityPolicyReviews struct {
	c  *Client
	ns string
}

// newPodSecurityPolicyReviews returns a podSecurityPolicyReviews
func newPodSecurityPolicyReviews(c *Client, namespace string) *podSecurityPolicyReviews {
	return &podSecurityPolicyReviews{
		c:  c,
		ns: namespace,
	}
}

// Create creates a PodSecurityPolicyReview
func (p *podSecurityPolicyReviews) Create(pspr *securityapi.PodSecurityPolicyReview) (result *securityapi.PodSecurityPolicyReview, err error) {
	result = &securityapi.PodSecurityPolicyReview{}
	err = p.c.Post().Namespace(p.ns).Resource("podSecurityPolicyReviews").Body(pspr).Do().Into(result)
	return
}
