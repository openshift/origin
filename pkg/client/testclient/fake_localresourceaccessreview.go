package testclient

import (
	ktestclient "k8s.io/kubernetes/pkg/client/testclient"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

type FakeLocalResourceAccessReviews struct {
	Fake      *Fake
	Namespace string
}

func (c *FakeLocalResourceAccessReviews) Create(inObj *authorizationapi.LocalResourceAccessReview) (*authorizationapi.ResourceAccessReviewResponse, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewCreateAction("localresourceaccessreviews", c.Namespace, inObj), &authorizationapi.ResourceAccessReviewResponse{})
	if cast, ok := obj.(*authorizationapi.ResourceAccessReviewResponse); ok {
		return cast, err
	}
	return nil, err
}
