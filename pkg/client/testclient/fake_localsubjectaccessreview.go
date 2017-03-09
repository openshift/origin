package testclient

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kubernetes/pkg/client/testing/core"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

type FakeLocalSubjectAccessReviews struct {
	Fake      *Fake
	Namespace string
}

var localSubjectAccessReviewsResource = schema.GroupVersionResource{Group: "", Version: "", Resource: "localsubjectaccessreviews"}

func (c *FakeLocalSubjectAccessReviews) Create(inObj *authorizationapi.LocalSubjectAccessReview) (*authorizationapi.SubjectAccessReviewResponse, error) {
	obj, err := c.Fake.Invokes(core.NewCreateAction(localSubjectAccessReviewsResource, c.Namespace, inObj), &authorizationapi.SubjectAccessReviewResponse{})
	if cast, ok := obj.(*authorizationapi.SubjectAccessReviewResponse); ok {
		return cast, err
	}
	return nil, err
}
