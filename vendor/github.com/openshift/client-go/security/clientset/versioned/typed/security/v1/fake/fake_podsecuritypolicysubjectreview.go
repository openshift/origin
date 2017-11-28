package fake

import (
	v1 "github.com/openshift/api/security/v1"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	testing "k8s.io/client-go/testing"
)

// FakePodSecurityPolicySubjectReviews implements PodSecurityPolicySubjectReviewInterface
type FakePodSecurityPolicySubjectReviews struct {
	Fake *FakeSecurityV1
	ns   string
}

var podsecuritypolicysubjectreviewsResource = schema.GroupVersionResource{Group: "security.openshift.io", Version: "v1", Resource: "podsecuritypolicysubjectreviews"}

var podsecuritypolicysubjectreviewsKind = schema.GroupVersionKind{Group: "security.openshift.io", Version: "v1", Kind: "PodSecurityPolicySubjectReview"}

// Create takes the representation of a podSecurityPolicySubjectReview and creates it.  Returns the server's representation of the podSecurityPolicySubjectReview, and an error, if there is any.
func (c *FakePodSecurityPolicySubjectReviews) Create(podSecurityPolicySubjectReview *v1.PodSecurityPolicySubjectReview) (result *v1.PodSecurityPolicySubjectReview, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(podsecuritypolicysubjectreviewsResource, c.ns, podSecurityPolicySubjectReview), &v1.PodSecurityPolicySubjectReview{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.PodSecurityPolicySubjectReview), err
}
