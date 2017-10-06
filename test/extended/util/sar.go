package util

import (
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	authorizationapiv1 "k8s.io/kubernetes/pkg/apis/authorization/v1"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/clientset"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

func WaitForSelfSAR(interval, timeout time.Duration, c kclientset.Interface, selfSAR authorizationapiv1.SelfSubjectAccessReviewSpec) error {
	err := wait.PollImmediate(interval, timeout, func() (bool, error) {
		res, err := c.AuthorizationV1().SelfSubjectAccessReviews().Create(
			&authorizationapiv1.SelfSubjectAccessReview{
				Spec: selfSAR,
			},
		)
		if err != nil {
			return false, err
		}

		if !res.Status.Allowed {
			e2e.Logf("Waiting for SelfSAR (ResourceAttributes: %#v, NonResourceAttributes: %#v) to be allowed, current Status: %#v", selfSAR.ResourceAttributes, selfSAR.NonResourceAttributes, res.Status)
			return false, nil
		}

		return true, nil
	})
	if err != nil {
		return fmt.Errorf("failed to wait for SelfSAR (ResourceAttributes: %#v, NonResourceAttributes: %#v), err: %v", selfSAR.ResourceAttributes, selfSAR.NonResourceAttributes, err)
	}

	return nil
}
