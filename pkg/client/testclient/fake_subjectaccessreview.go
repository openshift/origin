package testclient

import (
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/client/testing/core"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

// FakeClusterSubjectAccessReviews implements the ClusterSubjectAccessReviews interface.
// Meant to be embedded into a struct to get a default implementation.
// This makes faking out just the methods you want to test easier.
type FakeClusterSubjectAccessReviews struct {
	Fake *Fake
}

var subjectAccessReviewsResource = unversioned.GroupVersionResource{Group: "", Version: "", Resource: "subjectaccessreviews"}

func (c *FakeClusterSubjectAccessReviews) Create(inObj *authorizationapi.SubjectAccessReview) (*authorizationapi.SubjectAccessReviewResponse, error) {
	obj, err := c.Fake.Invokes(core.NewRootCreateAction(subjectAccessReviewsResource, inObj), &authorizationapi.SubjectAccessReviewResponse{})
	if cast, ok := obj.(*authorizationapi.SubjectAccessReviewResponse); ok {
		return cast, err
	}
	return nil, err
}
