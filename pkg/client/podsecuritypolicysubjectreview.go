package client

import securityapi "github.com/openshift/origin/pkg/security/api"

// PodSecurityPolicySubjectReviewsNamespacer has methods to work with PodSecurityPolicySubjectReview resources in the cluster scope
type PodSecurityPolicySubjectReviewsNamespacer interface {
	PodSecurityPolicySubjectReviews(namespace string) PodSecurityPolicySubjectReviewInterface
}

// PodSecurityPolicySubjectReviewInterface exposes methods on PodSecurityPolicySubjectReview resources.
type PodSecurityPolicySubjectReviewInterface interface {
	Create(policy *securityapi.PodSecurityPolicySubjectReview) (*securityapi.PodSecurityPolicySubjectReview, error)
}

// PodSecurityPolicySubjectReviews implements PodSecurityPolicySubjectReviews interface
type podSecurityPolicySubjectReviews struct {
	c  *Client
	ns string
}

// newPodSecurityPolicySubjectReviews returns a PodSecurityPolicySubjectReviews
func newPodSecurityPolicySubjectReviews(c *Client, namespace string) *podSecurityPolicySubjectReviews {
	return &podSecurityPolicySubjectReviews{
		c:  c,
		ns: namespace,
	}
}

func (p *podSecurityPolicySubjectReviews) Create(pspsr *securityapi.PodSecurityPolicySubjectReview) (result *securityapi.PodSecurityPolicySubjectReview, err error) {
	result = &securityapi.PodSecurityPolicySubjectReview{}
	err = p.c.Post().Namespace(p.ns).Resource("podSecurityPolicySubjectReviews").Body(pspsr).Do().Into(result)
	return
}

// PodSecurityPolicySelfSubjectReviewsNamespacer has methods to work with PodSecurityPolicySelfSubjectReview resources in the cluster scope
type PodSecurityPolicySelfSubjectReviewsNamespacer interface {
	PodSecurityPolicySelfSubjectReviews(namespace string) PodSecurityPolicySelfSubjectReviewInterface
}

// PodSecurityPolicySelfSubjectReviewInterface exposes methods on PodSecurityPolicySelfSubjectReview resources.
type PodSecurityPolicySelfSubjectReviewInterface interface {
	Create(policy *securityapi.PodSecurityPolicySelfSubjectReview) (*securityapi.PodSecurityPolicySelfSubjectReview, error)
}

type podSecurityPolicySelfSubjectReviews struct {
	c  *Client
	ns string
}

func newPodSecurityPolicySelfSubjectReviews(c *Client, namespace string) *podSecurityPolicySelfSubjectReviews {
	return &podSecurityPolicySelfSubjectReviews{
		c:  c,
		ns: namespace,
	}
}

func (p *podSecurityPolicySelfSubjectReviews) Create(pspssr *securityapi.PodSecurityPolicySelfSubjectReview) (result *securityapi.PodSecurityPolicySelfSubjectReview, err error) {
	result = &securityapi.PodSecurityPolicySelfSubjectReview{}
	err = p.c.Post().Namespace(p.ns).Resource("podSecurityPolicySelfSubjectReviews").Body(pspssr).Do().Into(result)
	return
}
