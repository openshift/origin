package testclient

import (
	securityapi "github.com/openshift/origin/pkg/security/apis/security"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgotesting "k8s.io/client-go/testing"
)

// FakePodSecurityPolicySubjectReviews implements the PodSecurityPolicySubjectReviews interface.
// Meant to be embedded into a struct to get a default implementation.
// This makes faking out just the methods you want to test easier.
type FakePodSecurityPolicySubjectReviews struct {
	Fake      *Fake
	Namespace string
}

var podSecurityPolicySubjectReviewsResource = schema.GroupVersionResource{Group: "", Version: "", Resource: "podsecuritypolicysubjectreviews"}

func (c *FakePodSecurityPolicySubjectReviews) Create(inObj *securityapi.PodSecurityPolicySubjectReview) (*securityapi.PodSecurityPolicySubjectReview, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewCreateAction(podSecurityPolicySubjectReviewsResource, c.Namespace, inObj), &securityapi.PodSecurityPolicySubjectReview{})
	if cast, ok := obj.(*securityapi.PodSecurityPolicySubjectReview); ok {
		return cast, err
	}
	return nil, err
}

// FakePodSecurityPolicySelfSubjectReviews implements the PodSecurityPolicySelfSubjectReviews interface.
// Meant to be embedded into a struct to get a default implementation.
// This makes faking out just the methods you want to test easier.
type FakePodSecurityPolicySelfSubjectReviews struct {
	Fake      *Fake
	Namespace string
}

var podSecurityPolicySelfSubjectReviewsResource = schema.GroupVersionResource{Group: "", Version: "", Resource: "podsecuritypolicyselfsubjectreviews"}

func (c *FakePodSecurityPolicySelfSubjectReviews) Create(inObj *securityapi.PodSecurityPolicySelfSubjectReview) (*securityapi.PodSecurityPolicySelfSubjectReview, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewCreateAction(podSecurityPolicySelfSubjectReviewsResource, c.Namespace, inObj), &securityapi.PodSecurityPolicySelfSubjectReview{})
	if cast, ok := obj.(*securityapi.PodSecurityPolicySelfSubjectReview); ok {
		return cast, err
	}
	return nil, err
}
