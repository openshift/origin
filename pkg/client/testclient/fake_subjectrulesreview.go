package testclient

import (
	"k8s.io/kubernetes/pkg/client/testing/core"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

type FakeSubjectRulesReviews struct {
	Fake      *Fake
	Namespace string
}

func (c *FakeSubjectRulesReviews) Create(inObj *authorizationapi.SubjectRulesReview) (*authorizationapi.SubjectRulesReview, error) {
	obj, err := c.Fake.Invokes(core.NewCreateAction(selfSubjectRulesReviewsResource, c.Namespace, inObj), &authorizationapi.SubjectRulesReview{})
	if cast, ok := obj.(*authorizationapi.SubjectRulesReview); ok {
		return cast, err
	}
	return nil, err
}
