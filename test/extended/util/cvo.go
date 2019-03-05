package util

import (
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	configv1 "github.com/openshift/api/config/v1"
	configclientv1 "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"
)

// WaitForClusterProgression waits for a cluster-level configuration change to propagate across all operators.
// This polls all ClusterOperator objects and waits until the following states are stable:
//
// 1. Available = true
// 2. Progressing = false
// 3. Failing = false
func WaitForClusterProgression(cv configclientv1.ClusterOperatorsGetter, timeout time.Duration) error {
	success := 0
	return wait.Poll(10*time.Second, timeout, func() (bool, error) {
		clusterOperators, err := cv.ClusterOperators().List(metav1.ListOptions{})
		if err != nil {
			success = 0
			// Transient client-side timeout
			if errors.IsTimeout(err) {
				e2e.Logf("Timeout listing cluster operators")
				return false, nil
			}
			// Wait if underlying service is unavailable or times out on server side - indicates apiserver churn
			if errors.IsServiceUnavailable(err) || errors.IsServerTimeout(err) {
				e2e.Logf("API servers are unavailable or timing out.")
				return false, nil
			}
			return false, err
		}
		for _, op := range clusterOperators.Items {
			for _, status := range op.Status.Conditions {
				switch status.Type {
				case configv1.OperatorFailing:
					if status.Status == configv1.ConditionTrue {
						// Operator is failing - can happen if API servers crash on MCO rollout
						success = 0
						e2e.Logf("Operator %s is failing due to %s: %s", op.Name, status.Reason, status.Message)
						return false, nil
					}
				case configv1.OperatorAvailable:
					if status.Status == configv1.ConditionFalse {
						// Operator is not available - not ready
						success = 0
						e2e.Logf("Waiting for operator %s to become available", op.Name)
						return false, nil
					}
				case configv1.OperatorProgressing:
					if status.Status == configv1.ConditionTrue {
						// Operator is progressing - not ready
						success = 0
						e2e.Logf("Waiting for operator %s to finish progress on %s: %s", op.Name, status.Reason, status.Message)
						return false, nil
					}
				}
			}
		}
		success++
		return success > 2, nil
	})

}
