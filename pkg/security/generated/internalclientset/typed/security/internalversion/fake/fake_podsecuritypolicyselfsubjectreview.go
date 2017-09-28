package fake

import (
	security "github.com/openshift/origin/pkg/security/apis/security"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	testing "k8s.io/client-go/testing"
)

// FakePodSecurityPolicySelfSubjectReviews implements PodSecurityPolicySelfSubjectReviewInterface
type FakePodSecurityPolicySelfSubjectReviews struct {
	Fake *FakeSecurity
	ns   string
}

var podsecuritypolicyselfsubjectreviewsResource = schema.GroupVersionResource{Group: "security.openshift.io", Version: "", Resource: "podsecuritypolicyselfsubjectreviews"}

var podsecuritypolicyselfsubjectreviewsKind = schema.GroupVersionKind{Group: "security.openshift.io", Version: "", Kind: "PodSecurityPolicySelfSubjectReview"}

// Create takes the representation of a podSecurityPolicySelfSubjectReview and creates it.  Returns the server's representation of the podSecurityPolicySelfSubjectReview, and an error, if there is any.
func (c *FakePodSecurityPolicySelfSubjectReviews) Create(podSecurityPolicySelfSubjectReview *security.PodSecurityPolicySelfSubjectReview) (result *security.PodSecurityPolicySelfSubjectReview, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(podsecuritypolicyselfsubjectreviewsResource, c.ns, podSecurityPolicySelfSubjectReview), &security.PodSecurityPolicySelfSubjectReview{})

	if obj == nil {
		return nil, err
	}
	return obj.(*security.PodSecurityPolicySelfSubjectReview), err
}
