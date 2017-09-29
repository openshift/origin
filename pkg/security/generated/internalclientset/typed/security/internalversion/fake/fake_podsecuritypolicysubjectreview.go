package fake

import (
	security "github.com/openshift/origin/pkg/security/apis/security"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	testing "k8s.io/client-go/testing"
)

// FakePodSecurityPolicySubjectReviews implements PodSecurityPolicySubjectReviewInterface
type FakePodSecurityPolicySubjectReviews struct {
	Fake *FakeSecurity
	ns   string
}

var podsecuritypolicysubjectreviewsResource = schema.GroupVersionResource{Group: "security.openshift.io", Version: "", Resource: "podsecuritypolicysubjectreviews"}

var podsecuritypolicysubjectreviewsKind = schema.GroupVersionKind{Group: "security.openshift.io", Version: "", Kind: "PodSecurityPolicySubjectReview"}

// Create takes the representation of a podSecurityPolicySubjectReview and creates it.  Returns the server's representation of the podSecurityPolicySubjectReview, and an error, if there is any.
func (c *FakePodSecurityPolicySubjectReviews) Create(podSecurityPolicySubjectReview *security.PodSecurityPolicySubjectReview) (result *security.PodSecurityPolicySubjectReview, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(podsecuritypolicysubjectreviewsResource, c.ns, podSecurityPolicySubjectReview), &security.PodSecurityPolicySubjectReview{})

	if obj == nil {
		return nil, err
	}
	return obj.(*security.PodSecurityPolicySubjectReview), err
}
