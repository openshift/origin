package fake

import (
	security "github.com/openshift/origin/pkg/security/apis/security"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	testing "k8s.io/client-go/testing"
)

// FakePodSecurityPolicyReviews implements PodSecurityPolicyReviewInterface
type FakePodSecurityPolicyReviews struct {
	Fake *FakeSecurity
	ns   string
}

var podsecuritypolicyreviewsResource = schema.GroupVersionResource{Group: "security.openshift.io", Version: "", Resource: "podsecuritypolicyreviews"}

var podsecuritypolicyreviewsKind = schema.GroupVersionKind{Group: "security.openshift.io", Version: "", Kind: "PodSecurityPolicyReview"}

// Create takes the representation of a podSecurityPolicyReview and creates it.  Returns the server's representation of the podSecurityPolicyReview, and an error, if there is any.
func (c *FakePodSecurityPolicyReviews) Create(podSecurityPolicyReview *security.PodSecurityPolicyReview) (result *security.PodSecurityPolicyReview, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(podsecuritypolicyreviewsResource, c.ns, podSecurityPolicyReview), &security.PodSecurityPolicyReview{})

	if obj == nil {
		return nil, err
	}
	return obj.(*security.PodSecurityPolicyReview), err
}
