package testclient

import (
	securityapi "github.com/openshift/origin/pkg/security/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/client/testing/core"
)

// FakePodSecurityPolicyReviews implements the PodSecurityPolicyReviews interface.
// Meant to be embedded into a struct to get a default implementation.
// This makes faking out just the methods you want to test easier.
type FakePodSecurityPolicyReviews struct {
	Fake      *Fake
	Namespace string
}

var podSecurityPolicyReviewsResource = unversioned.GroupVersionResource{Group: "", Version: "", Resource: "podsecuritypolicyreviews"}

func (c *FakePodSecurityPolicyReviews) Create(inObj *securityapi.PodSecurityPolicyReview) (*securityapi.PodSecurityPolicyReview, error) {
	obj, err := c.Fake.Invokes(core.NewCreateAction(podSecurityPolicyReviewsResource, c.Namespace, inObj), &securityapi.PodSecurityPolicyReview{})
	if cast, ok := obj.(*securityapi.PodSecurityPolicyReview); ok {
		return cast, err
	}
	return nil, err
}
