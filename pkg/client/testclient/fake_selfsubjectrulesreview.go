package testclient

import (
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/client/testing/core"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

type FakeSelfSubjectRulesReviews struct {
	Fake      *Fake
	Namespace string
}

var selfSubjectRulesReviewsResource = unversioned.GroupVersionResource{Group: "", Version: "", Resource: "selfsubjectrulesreviews"}

func (c *FakeSelfSubjectRulesReviews) Create(inObj *authorizationapi.SelfSubjectRulesReview) (*authorizationapi.SelfSubjectRulesReview, error) {
	obj, err := c.Fake.Invokes(core.NewCreateAction(selfSubjectRulesReviewsResource, c.Namespace, inObj), &authorizationapi.SelfSubjectRulesReview{})
	if cast, ok := obj.(*authorizationapi.SelfSubjectRulesReview); ok {
		return cast, err
	}
	return nil, err
}
