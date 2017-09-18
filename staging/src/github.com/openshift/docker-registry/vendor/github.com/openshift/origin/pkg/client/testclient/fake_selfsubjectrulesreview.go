package testclient

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgotesting "k8s.io/client-go/testing"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
)

type FakeSelfSubjectRulesReviews struct {
	Fake      *Fake
	Namespace string
}

var selfSubjectRulesReviewsResource = schema.GroupVersionResource{Group: "", Version: "", Resource: "selfsubjectrulesreviews"}

func (c *FakeSelfSubjectRulesReviews) Create(inObj *authorizationapi.SelfSubjectRulesReview) (*authorizationapi.SelfSubjectRulesReview, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewCreateAction(selfSubjectRulesReviewsResource, c.Namespace, inObj), &authorizationapi.SelfSubjectRulesReview{})
	if cast, ok := obj.(*authorizationapi.SelfSubjectRulesReview); ok {
		return cast, err
	}
	return nil, err
}
