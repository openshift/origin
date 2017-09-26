package util

import (
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	authorizationtypedclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/authorization/internalversion"

	"k8s.io/kubernetes/pkg/apis/authorization"
)

const (
	PolicyCachePollInterval = 100 * time.Millisecond
	PolicyCachePollTimeout  = 5 * time.Second
)

// WaitForPolicyUpdate checks if the given client can perform the named verb and action.
// If PolicyCachePollTimeout is reached without the expected condition matching, an error is returned
func WaitForPolicyUpdate(c authorizationtypedclient.SelfSubjectAccessReviewsGetter, namespace, verb string, resource schema.GroupResource, allowed bool) error {
	review := &authorization.SelfSubjectAccessReview{
		Spec: authorization.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &authorization.ResourceAttributes{
				Namespace: namespace,
				Verb:      verb,
				Group:     resource.Group,
				Resource:  resource.Resource,
			},
		},
	}
	err := wait.Poll(PolicyCachePollInterval, PolicyCachePollTimeout, func() (bool, error) {
		response, err := c.SelfSubjectAccessReviews().Create(review)
		if err != nil {
			return false, err
		}
		return response.Status.Allowed == allowed, nil
	})
	return err
}

// WaitForClusterPolicyUpdate checks if the given client can perform the named verb and action.
// If PolicyCachePollTimeout is reached without the expected condition matching, an error is returned
func WaitForClusterPolicyUpdate(c authorizationtypedclient.SelfSubjectAccessReviewsGetter, verb string, resource schema.GroupResource, allowed bool) error {
	review := &authorization.SelfSubjectAccessReview{
		Spec: authorization.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &authorization.ResourceAttributes{
				Verb:     verb,
				Group:    resource.Group,
				Resource: resource.Resource,
			},
		},
	}
	err := wait.Poll(PolicyCachePollInterval, PolicyCachePollTimeout, func() (bool, error) {
		response, err := c.SelfSubjectAccessReviews().Create(review)
		if err != nil {
			return false, err
		}
		if response.Status.Allowed != allowed {
			return false, nil
		}
		return true, nil
	})
	return err
}
