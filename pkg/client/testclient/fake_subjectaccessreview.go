package testclient

import (
	ktestclient "k8s.io/kubernetes/pkg/client/testclient"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

// FakeClusterSubjectAccessReviews implements the ClusterSubjectAccessReviews interface.
// Meant to be embedded into a struct to get a default implementation.
// This makes faking out just the methods you want to test easier.
type FakeClusterSubjectAccessReviews struct {
	Fake *Fake
}

func (c *FakeClusterSubjectAccessReviews) Create(inObj *authorizationapi.SubjectAccessReview) (*authorizationapi.SubjectAccessReviewResponse, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewRootCreateAction("subjectaccessreviews", inObj), &authorizationapi.SubjectAccessReviewResponse{})
	if cast, ok := obj.(*authorizationapi.SubjectAccessReviewResponse); ok {
		return cast, err
	}
	return nil, err
}
