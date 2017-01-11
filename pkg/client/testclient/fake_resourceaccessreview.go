package testclient

import (
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/client/testing/core"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

type FakeClusterResourceAccessReviews struct {
	Fake *Fake
}

var resourceAccessReviewsResource = unversioned.GroupVersionResource{Group: "", Version: "", Resource: "resourceaccessreviews"}

func (c *FakeClusterResourceAccessReviews) Create(inObj *authorizationapi.ResourceAccessReview) (*authorizationapi.ResourceAccessReviewResponse, error) {
	obj, err := c.Fake.Invokes(core.NewRootCreateAction(resourceAccessReviewsResource, inObj), &authorizationapi.ResourceAccessReviewResponse{})
	if cast, ok := obj.(*authorizationapi.ResourceAccessReviewResponse); ok {
		return cast, err
	}
	return nil, err
}
