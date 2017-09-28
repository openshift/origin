package fake

import (
	authorization "github.com/openshift/origin/pkg/authorization/apis/authorization"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	testing "k8s.io/client-go/testing"
)

// FakeResourceAccessReviews implements ResourceAccessReviewInterface
type FakeResourceAccessReviews struct {
	Fake *FakeAuthorization
}

var resourceaccessreviewsResource = schema.GroupVersionResource{Group: "authorization.openshift.io", Version: "", Resource: "resourceaccessreviews"}

var resourceaccessreviewsKind = schema.GroupVersionKind{Group: "authorization.openshift.io", Version: "", Kind: "ResourceAccessReview"}

// Create takes the representation of a resourceAccessReview and creates it.  Returns the server's representation of the resourceAccessReviewResponse, and an error, if there is any.
func (c *FakeResourceAccessReviews) Create(resourceAccessReview *authorization.ResourceAccessReview) (result *authorization.ResourceAccessReviewResponse, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(resourceaccessreviewsResource, resourceAccessReview), &authorization.ResourceAccessReviewResponse{})
	if obj == nil {
		return nil, err
	}
	return obj.(*authorization.ResourceAccessReviewResponse), err
}
