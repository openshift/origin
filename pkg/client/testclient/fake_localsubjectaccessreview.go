package testclient

import (
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/client/testing/core"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

type FakeLocalSubjectAccessReviews struct {
	Fake      *Fake
	Namespace string
}

var localSubjectAccessReviewsResource = unversioned.GroupVersionResource{Group: "", Version: "", Resource: "localsubjectaccessreviews"}

func (c *FakeLocalSubjectAccessReviews) Create(inObj *authorizationapi.LocalSubjectAccessReview) (*authorizationapi.SubjectAccessReviewResponse, error) {
	obj, err := c.Fake.Invokes(core.NewCreateAction(localSubjectAccessReviewsResource, c.Namespace, inObj), &authorizationapi.SubjectAccessReviewResponse{})
	if cast, ok := obj.(*authorizationapi.SubjectAccessReviewResponse); ok {
		return cast, err
	}
	return nil, err
}
