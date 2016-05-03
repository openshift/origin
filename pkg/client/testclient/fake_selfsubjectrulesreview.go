package testclient

import (
	ktestclient "k8s.io/kubernetes/pkg/client/unversioned/testclient"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

type FakeSelfSubjectRulesReviews struct {
	Fake      *Fake
	Namespace string
}

func (c *FakeSelfSubjectRulesReviews) Create(inObj *authorizationapi.SelfSubjectRulesReview) (*authorizationapi.SelfSubjectRulesReview, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewCreateAction("selfsubjectrulesreviews", c.Namespace, inObj), &authorizationapi.SelfSubjectRulesReview{})
	if cast, ok := obj.(*authorizationapi.SelfSubjectRulesReview); ok {
		return cast, err
	}
	return nil, err
}
