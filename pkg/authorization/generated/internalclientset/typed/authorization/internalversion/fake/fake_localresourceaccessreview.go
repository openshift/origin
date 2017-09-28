package fake

import (
	authorization "github.com/openshift/origin/pkg/authorization/apis/authorization"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	testing "k8s.io/client-go/testing"
)

// FakeLocalResourceAccessReviews implements LocalResourceAccessReviewInterface
type FakeLocalResourceAccessReviews struct {
	Fake *FakeAuthorization
	ns   string
}

var localresourceaccessreviewsResource = schema.GroupVersionResource{Group: "authorization.openshift.io", Version: "", Resource: "localresourceaccessreviews"}

var localresourceaccessreviewsKind = schema.GroupVersionKind{Group: "authorization.openshift.io", Version: "", Kind: "LocalResourceAccessReview"}

// Create takes the representation of a localResourceAccessReview and creates it.  Returns the server's representation of the resourceAccessReviewResponse, and an error, if there is any.
func (c *FakeLocalResourceAccessReviews) Create(localResourceAccessReview *authorization.LocalResourceAccessReview) (result *authorization.ResourceAccessReviewResponse, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(localresourceaccessreviewsResource, c.ns, localResourceAccessReview), &authorization.ResourceAccessReviewResponse{})

	if obj == nil {
		return nil, err
	}
	return obj.(*authorization.ResourceAccessReviewResponse), err
}
