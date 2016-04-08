package client

import securityapi "github.com/openshift/origin/pkg/security/api"

// PodSpecReviews has methods to work with PodSpecReview resources in the cluster scope
type PodSpecReviews interface {
	PodSpecReviews() PodSpecReviewInterface
}

// PodSpecReviewInterface exposes methods on PodSpecReview resources.
type PodSpecReviewInterface interface {
	Create(policy *securityapi.PodSpecReview) (*securityapi.PodSpecReview, error)
}

// podSpecReviews implements PodSpecReviews interface
type podSpecReviews struct {
	r     *Client
	token *string
}

// newPodSpecReviews returns a podSpecReviews
func newPodSpecReviews(c *Client) *podSpecReviews {
	return &podSpecReviews{
		r: c,
	}
}

// Create creates a PodSpecReview
func (p *podSpecReviews) Create(sar *securityapi.PodSpecReview) (*securityapi.PodSpecReview, error) {
	result := &securityapi.PodSpecReview{}

	return result, nil
}
