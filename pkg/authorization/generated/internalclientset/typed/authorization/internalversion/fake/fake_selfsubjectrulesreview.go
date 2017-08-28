package fake

import (
	authorization "github.com/openshift/origin/pkg/authorization/apis/authorization"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	testing "k8s.io/client-go/testing"
)

// FakeSelfSubjectRulesReviews implements SelfSubjectRulesReviewInterface
type FakeSelfSubjectRulesReviews struct {
	Fake *FakeAuthorization
	ns   string
}

var selfsubjectrulesreviewsResource = schema.GroupVersionResource{Group: "authorization.openshift.io", Version: "", Resource: "selfsubjectrulesreviews"}

var selfsubjectrulesreviewsKind = schema.GroupVersionKind{Group: "authorization.openshift.io", Version: "", Kind: "SelfSubjectRulesReview"}

// Create takes the representation of a selfSubjectRulesReview and creates it.  Returns the server's representation of the selfSubjectRulesReview, and an error, if there is any.
func (c *FakeSelfSubjectRulesReviews) Create(selfSubjectRulesReview *authorization.SelfSubjectRulesReview) (result *authorization.SelfSubjectRulesReview, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(selfsubjectrulesreviewsResource, c.ns, selfSubjectRulesReview), &authorization.SelfSubjectRulesReview{})

	if obj == nil {
		return nil, err
	}
	return obj.(*authorization.SelfSubjectRulesReview), err
}
