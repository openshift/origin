package client

import (
	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

type FakeSubjectAccessReviews struct {
	Fake *Fake
}

func (c *FakeSubjectAccessReviews) Create(subjectAccessReview *authorizationapi.SubjectAccessReview) (*authorizationapi.SubjectAccessReviewResponse, error) {
	obj, err := c.Fake.Invokes(FakeAction{Action: "create-subjectAccessReview", Value: subjectAccessReview}, &authorizationapi.SubjectAccessReviewResponse{})
	return obj.(*authorizationapi.SubjectAccessReviewResponse), err
}
