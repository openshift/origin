package testclient

import (
	ktestclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client/testclient"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

// FakeSubjectAccessReviews implements SubjectAccessReviewInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeSubjectAccessReviews struct {
	Fake *Fake
}

func (c *FakeSubjectAccessReviews) Create(subjectAccessReview *authorizationapi.SubjectAccessReview) (*authorizationapi.SubjectAccessReviewResponse, error) {
	obj, err := c.Fake.Invokes(ktestclient.FakeAction{Action: "create-subjectAccessReview", Value: subjectAccessReview}, &authorizationapi.SubjectAccessReviewResponse{})
	return obj.(*authorizationapi.SubjectAccessReviewResponse), err
}

// FakeClusterSubjectAccessReviews implements the ClusterSubjectAccessReviews interface.
// Meant to be embedded into a struct to get a default implementation.
// This makes faking out just the methods you want to test easier.
type FakeClusterSubjectAccessReviews struct {
	Fake *Fake
}

func (c *FakeClusterSubjectAccessReviews) Create(resourceAccessReview *authorizationapi.SubjectAccessReview) (*authorizationapi.SubjectAccessReviewResponse, error) {
	obj, err := c.Fake.Invokes(ktestclient.FakeAction{Action: "create-cluster-subjectAccessReview", Value: resourceAccessReview}, &authorizationapi.SubjectAccessReviewResponse{})
	return obj.(*authorizationapi.SubjectAccessReviewResponse), err
}
