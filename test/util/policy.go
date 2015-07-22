package util

import (
	"time"

	"k8s.io/kubernetes/pkg/util/wait"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/client"
)

const (
	PolicyCachePollInterval = 100 * time.Millisecond
	PolicyCachePollTimeout  = 5 * time.Second
)

// WaitForPolicyUpdate checks if the given client can perform the named verb and action.
// If PolicyCachePollTimeout is reached without the expected condition matching, an error is returned
func WaitForPolicyUpdate(c *client.Client, namespace, verb, resource string, allowed bool) error {
	review := &authorizationapi.LocalSubjectAccessReview{Action: authorizationapi.AuthorizationAttributes{Verb: verb, Resource: resource}}
	err := wait.Poll(PolicyCachePollInterval, PolicyCachePollTimeout, func() (bool, error) {
		response, err := c.LocalSubjectAccessReviews(namespace).Create(review)
		if err != nil {
			return false, err
		}
		if response.Allowed != allowed {
			return false, nil
		}
		return true, nil
	})
	return err
}
