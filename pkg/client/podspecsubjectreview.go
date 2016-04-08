package client

import securityapi "github.com/openshift/origin/pkg/security/api"

// PodSpecSubjectReviews has methods to work with PodSpecSubjectReview resources in the cluster scope
type PodSpecSubjectReviews interface {
	PodSpecSubjectReviews() PodSpecSubjectReviewInterface
}

// PodSpecSubjectReviewInterface exposes methods on PodSpecSubjectReview resources.
type PodSpecSubjectReviewInterface interface {
	Create(policy *securityapi.PodSpecSubjectReview) (*securityapi.PodSpecSubjectReview, error)
}

// podSpecSubjectReviews implements PodSpecSubjectReviews interface
type podSpecSubjectReviews struct {
	r     *Client
	token *string
}

// newPodSpecSubjectReviews returns a podSpecSubjectReviews
func newPodSpecSubjectReviews(c *Client) *podSpecSubjectReviews {
	return &podSpecSubjectReviews{
		r: c,
	}
}

func (p *podSpecSubjectReviews) Create(sar *securityapi.PodSpecSubjectReview) (*securityapi.PodSpecSubjectReview, error) {
	result := &securityapi.PodSpecSubjectReview{}

	return result, nil
}

// PodSpecSelfSubjectReviews has methods to work with PodSpecSelfSubjectReview resources in the cluster scope
type PodSpecSelfSubjectReviews interface {
	PodSpecSelfSubjectReviews() PodSpecSelfSubjectReviewInterface
}

// PodSpecSelfSubjectReviewInterface exposes methods on PodSpecSelfSubjectReview resources.
type PodSpecSelfSubjectReviewInterface interface {
	Create(policy *securityapi.PodSpecSelfSubjectReview) (*securityapi.PodSpecSelfSubjectReview, error)
}

// podSpecSelfSubjectReviews implements PodSpecSelfSubjectReviews interface
type podSpecSelfSubjectReviews struct {
	r     *Client
	token *string
}

// newPodSpecSelfSubjectReviews returns a podSpecSelfSubjectReviews
func newPodSpecSelfSubjectReviews(c *Client) *podSpecSelfSubjectReviews {
	return &podSpecSelfSubjectReviews{
		r: c,
	}
}

func (p *podSpecSelfSubjectReviews) Create(sar *securityapi.PodSpecSelfSubjectReview) (*securityapi.PodSpecSelfSubjectReview, error) {
	result := &securityapi.PodSpecSelfSubjectReview{}

	return result, nil
}
