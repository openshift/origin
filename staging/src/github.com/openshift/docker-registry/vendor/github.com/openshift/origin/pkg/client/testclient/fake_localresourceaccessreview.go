package testclient

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgotesting "k8s.io/client-go/testing"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
)

type FakeLocalResourceAccessReviews struct {
	Fake      *Fake
	Namespace string
}

var localResourceAccessReviewsResource = schema.GroupVersionResource{Group: "", Version: "", Resource: "localresourceaccessreviews"}

func (c *FakeLocalResourceAccessReviews) Create(inObj *authorizationapi.LocalResourceAccessReview) (*authorizationapi.ResourceAccessReviewResponse, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewCreateAction(localResourceAccessReviewsResource, c.Namespace, inObj), &authorizationapi.ResourceAccessReviewResponse{})
	if cast, ok := obj.(*authorizationapi.ResourceAccessReviewResponse); ok {
		return cast, err
	}
	return nil, err
}
