package client

import (
	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

type FakeResourceAccessReviews struct {
	Fake *Fake
}

func (c *FakeResourceAccessReviews) Create(resourceAccessReview *authorizationapi.ResourceAccessReview) (*authorizationapi.ResourceAccessReviewResponse, error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "create-resourceAccessReview", Value: resourceAccessReview})
	return &authorizationapi.ResourceAccessReviewResponse{}, nil
}

type FakeRootResourceAccessReviews struct {
	Fake *Fake
}

func (c *FakeRootResourceAccessReviews) Create(resourceAccessReview *authorizationapi.ResourceAccessReview) (*authorizationapi.ResourceAccessReviewResponse, error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "create-root-resourceAccessReview", Value: resourceAccessReview})
	return &authorizationapi.ResourceAccessReviewResponse{}, nil
}
