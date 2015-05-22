package testclient

import (
	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

// FakeResourceAccessReviews implements ResourceAccessReviewInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeResourceAccessReviews struct {
	Fake *Fake
}

func (c *FakeResourceAccessReviews) Create(resourceAccessReview *authorizationapi.ResourceAccessReview) (*authorizationapi.ResourceAccessReviewResponse, error) {
	obj, err := c.Fake.Invokes(FakeAction{Action: "create-resourceAccessReview", Value: resourceAccessReview}, &authorizationapi.ResourceAccessReviewResponse{})
	return obj.(*authorizationapi.ResourceAccessReviewResponse), err
}

type FakeClusterResourceAccessReviews struct {
	Fake *Fake
}

func (c *FakeClusterResourceAccessReviews) Create(resourceAccessReview *authorizationapi.ResourceAccessReview) (*authorizationapi.ResourceAccessReviewResponse, error) {
	obj, err := c.Fake.Invokes(FakeAction{Action: "create-cluster-resourceAccessReview", Value: resourceAccessReview}, &authorizationapi.ResourceAccessReviewResponse{})
	return obj.(*authorizationapi.ResourceAccessReviewResponse), err
}
