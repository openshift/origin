package testclient

import (
	ktestclient "k8s.io/kubernetes/pkg/client/unversioned/testclient"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

type FakeSubjectRulesReviews struct {
	Fake      *Fake
	Namespace string
}

func (c *FakeSubjectRulesReviews) Create(inObj *authorizationapi.SubjectRulesReview) (*authorizationapi.SubjectRulesReview, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewCreateAction("selfsubjectrulesreviews", c.Namespace, inObj), &authorizationapi.SubjectRulesReview{})
	if cast, ok := obj.(*authorizationapi.SubjectRulesReview); ok {
		return cast, err
	}
	return nil, err
}
