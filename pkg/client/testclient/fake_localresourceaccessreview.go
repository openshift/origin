package testclient

import (
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/client/testing/core"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

type FakeLocalResourceAccessReviews struct {
	Fake      *Fake
	Namespace string
}

var localResourceAccessReviewsResource = unversioned.GroupVersionResource{Group: "", Version: "", Resource: "localresourceaccessreviews"}

func (c *FakeLocalResourceAccessReviews) Create(inObj *authorizationapi.LocalResourceAccessReview) (*authorizationapi.ResourceAccessReviewResponse, error) {
	obj, err := c.Fake.Invokes(core.NewCreateAction(localResourceAccessReviewsResource, c.Namespace, inObj), &authorizationapi.ResourceAccessReviewResponse{})
	if cast, ok := obj.(*authorizationapi.ResourceAccessReviewResponse); ok {
		return cast, err
	}
	return nil, err
}
