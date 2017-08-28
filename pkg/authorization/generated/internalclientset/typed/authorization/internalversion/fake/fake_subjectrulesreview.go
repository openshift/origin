package fake

import (
	authorization "github.com/openshift/origin/pkg/authorization/apis/authorization"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	testing "k8s.io/client-go/testing"
)

// FakeSubjectRulesReviews implements SubjectRulesReviewInterface
type FakeSubjectRulesReviews struct {
	Fake *FakeAuthorization
	ns   string
}

var subjectrulesreviewsResource = schema.GroupVersionResource{Group: "authorization.openshift.io", Version: "", Resource: "subjectrulesreviews"}

var subjectrulesreviewsKind = schema.GroupVersionKind{Group: "authorization.openshift.io", Version: "", Kind: "SubjectRulesReview"}

// Create takes the representation of a subjectRulesReview and creates it.  Returns the server's representation of the subjectRulesReview, and an error, if there is any.
func (c *FakeSubjectRulesReviews) Create(subjectRulesReview *authorization.SubjectRulesReview) (result *authorization.SubjectRulesReview, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(subjectrulesreviewsResource, c.ns, subjectRulesReview), &authorization.SubjectRulesReview{})

	if obj == nil {
		return nil, err
	}
	return obj.(*authorization.SubjectRulesReview), err
}
