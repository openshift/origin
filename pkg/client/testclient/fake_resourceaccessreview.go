package testclient

import (
	ktestclient "k8s.io/kubernetes/pkg/client/testclient"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

// FakeResourceAccessReviews implements ResourceAccessReviewInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeResourceAccessReviews struct {
	Fake      *Fake
	Namespace string
}

func (c *FakeResourceAccessReviews) Create(inObj *authorizationapi.ResourceAccessReview) (*authorizationapi.ResourceAccessReviewResponse, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewCreateAction("resourceaccessreviews", c.Namespace, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*authorizationapi.ResourceAccessReviewResponse), err
}

type FakeClusterResourceAccessReviews struct {
	Fake *Fake
}

func (c *FakeClusterResourceAccessReviews) Create(inObj *authorizationapi.ResourceAccessReview) (*authorizationapi.ResourceAccessReviewResponse, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewRootCreateAction("resourceaccessreviews", inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*authorizationapi.ResourceAccessReviewResponse), err
}
