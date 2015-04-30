package client

import (
	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

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
