package compat_otp

import (
	"context"
	"fmt"
	"time"

	authorizationapiv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	kclientset "k8s.io/client-go/kubernetes"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

func WaitForSelfSAR(interval, timeout time.Duration, c kclientset.Interface, selfSAR authorizationapiv1.SelfSubjectAccessReviewSpec) error {
	err := wait.PollImmediate(interval, timeout, func() (bool, error) {
		res, err := c.AuthorizationV1().SelfSubjectAccessReviews().Create(context.Background(),
			&authorizationapiv1.SelfSubjectAccessReview{
				Spec: selfSAR,
			}, metav1.CreateOptions{})
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
