package testclient

import (
	ktestclient "k8s.io/kubernetes/pkg/client/unversioned/testclient"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

type FakeLocalSubjectAccessReviews struct {
	Fake      *Fake
	Namespace string
}

func (c *FakeLocalSubjectAccessReviews) Create(inObj *authorizationapi.LocalSubjectAccessReview) (*authorizationapi.SubjectAccessReviewResponse, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewCreateAction("localsubjectaccessreviews", c.Namespace, inObj), &authorizationapi.SubjectAccessReviewResponse{})
	if cast, ok := obj.(*authorizationapi.SubjectAccessReviewResponse); ok {
		return cast, err
	}
	return nil, err
}
