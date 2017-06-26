package testclient

import (
	clientgotesting "k8s.io/client-go/testing"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
)

type FakeSubjectRulesReviews struct {
	Fake      *Fake
	Namespace string
}

func (c *FakeSubjectRulesReviews) Create(inObj *authorizationapi.SubjectRulesReview) (*authorizationapi.SubjectRulesReview, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewCreateAction(selfSubjectRulesReviewsResource, c.Namespace, inObj), &authorizationapi.SubjectRulesReview{})
	if cast, ok := obj.(*authorizationapi.SubjectRulesReview); ok {
		return cast, err
	}
	return nil, err
}
