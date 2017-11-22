package fake

import (
	v1 "github.com/openshift/api/security/v1"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	testing "k8s.io/client-go/testing"
)

// FakePodSecurityPolicyReviews implements PodSecurityPolicyReviewInterface
type FakePodSecurityPolicyReviews struct {
	Fake *FakeSecurityV1
	ns   string
}

var podsecuritypolicyreviewsResource = schema.GroupVersionResource{Group: "security.openshift.io", Version: "v1", Resource: "podsecuritypolicyreviews"}

var podsecuritypolicyreviewsKind = schema.GroupVersionKind{Group: "security.openshift.io", Version: "v1", Kind: "PodSecurityPolicyReview"}

// Create takes the representation of a podSecurityPolicyReview and creates it.  Returns the server's representation of the podSecurityPolicyReview, and an error, if there is any.
func (c *FakePodSecurityPolicyReviews) Create(podSecurityPolicyReview *v1.PodSecurityPolicyReview) (result *v1.PodSecurityPolicyReview, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(podsecuritypolicyreviewsResource, c.ns, podSecurityPolicyReview), &v1.PodSecurityPolicyReview{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.PodSecurityPolicyReview), err
}
