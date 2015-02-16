package client

import (
	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

type FakeSubjectAccessReviews struct {
	Fake *Fake
}

func (c *FakeSubjectAccessReviews) Create(subjectAccessReview *authorizationapi.SubjectAccessReview) (*authorizationapi.SubjectAccessReviewResponse, error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "create-subjectAccessReview", Value: subjectAccessReview})
	return &authorizationapi.SubjectAccessReviewResponse{}, nil
}
