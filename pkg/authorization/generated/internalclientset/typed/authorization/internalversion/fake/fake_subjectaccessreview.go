package fake

import (
	authorization "github.com/openshift/origin/pkg/authorization/apis/authorization"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	testing "k8s.io/client-go/testing"
)

// FakeSubjectAccessReviews implements SubjectAccessReviewInterface
type FakeSubjectAccessReviews struct {
	Fake *FakeAuthorization
}

var subjectaccessreviewsResource = schema.GroupVersionResource{Group: "authorization.openshift.io", Version: "", Resource: "subjectaccessreviews"}

var subjectaccessreviewsKind = schema.GroupVersionKind{Group: "authorization.openshift.io", Version: "", Kind: "SubjectAccessReview"}

// Create takes the representation of a subjectAccessReview and creates it.  Returns the server's representation of the subjectAccessReviewResponse, and an error, if there is any.
func (c *FakeSubjectAccessReviews) Create(subjectAccessReview *authorization.SubjectAccessReview) (result *authorization.SubjectAccessReviewResponse, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(subjectaccessreviewsResource, subjectAccessReview), &authorization.SubjectAccessReviewResponse{})
	if obj == nil {
		return nil, err
	}
	return obj.(*authorization.SubjectAccessReviewResponse), err
}
