package testclient

import (
	securityapi "github.com/openshift/origin/pkg/security/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/client/testing/core"
)

// FakePodSecurityPolicySubjectReviews implements the PodSecurityPolicySubjectReviews interface.
// Meant to be embedded into a struct to get a default implementation.
// This makes faking out just the methods you want to test easier.
type FakePodSecurityPolicySubjectReviews struct {
	Fake      *Fake
	Namespace string
}

var podSecurityPolicySubjectReviewsResource = unversioned.GroupVersionResource{Group: "", Version: "", Resource: "podsecuritypolicysubjectreviews"}

func (c *FakePodSecurityPolicySubjectReviews) Create(inObj *securityapi.PodSecurityPolicySubjectReview) (*securityapi.PodSecurityPolicySubjectReview, error) {
	obj, err := c.Fake.Invokes(core.NewCreateAction(podSecurityPolicySubjectReviewsResource, c.Namespace, inObj), &securityapi.PodSecurityPolicySubjectReview{})
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

var podSecurityPolicySelfSubjectReviewsResource = unversioned.GroupVersionResource{Group: "", Version: "", Resource: "podsecuritypolicyselfsubjectreviews"}

func (c *FakePodSecurityPolicySelfSubjectReviews) Create(inObj *securityapi.PodSecurityPolicySelfSubjectReview) (*securityapi.PodSecurityPolicySelfSubjectReview, error) {
	obj, err := c.Fake.Invokes(core.NewCreateAction(podSecurityPolicySelfSubjectReviewsResource, c.Namespace, inObj), &securityapi.PodSecurityPolicySelfSubjectReview{})
	if cast, ok := obj.(*securityapi.PodSecurityPolicySelfSubjectReview); ok {
		return cast, err
	}
	return nil, err
}
