package util

import (
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	configv1 "github.com/openshift/api/config/v1"
	configclientv1 "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"
)

// WaitForClusterProgression waits for a cluster-level configuration change to propagate across all operators.
// This polls all ClusterVersion objects and waits until the following states are stable:
//
// 1. Available = true
// 2. Progressing = false
// 3. Failing = false
//
// If a ClusterVersion reports it is failing, this will abort with an error.
func WaitForClusterProgression(cv configclientv1.ClusterOperatorsGetter) error {
	success := 0
	return wait.Poll(10*time.Second, 10*time.Minute, func() (bool, error) {
		clusterOperators, err := cv.ClusterOperators().List(metav1.ListOptions{})
		if err != nil {
			success = 0
			// Wait if underlying service is unavailable - indicates apiserver churn
			if errors.IsServiceUnavailable(err) {
				return false, nil
			}
			return false, err
		}
		for _, op := range clusterOperators.Items {
			for _, status := range op.Status.Conditions {
				switch status.Type {
				case configv1.OperatorFailing:
					if status.Status == configv1.ConditionTrue {
						// Operator is failing - abort with error
						success = 0
						return false, fmt.Errorf("operator %s is failing due to %s: %s", op.Name, status.Reason, status.Message)
					}
				case configv1.OperatorAvailable:
					if status.Status == configv1.ConditionFalse {
						// Operator is not available - not ready
						success = 0
						return false, nil
					}
				case configv1.OperatorProgressing:
					if status.Status == configv1.ConditionTrue {
						// Operator is progressing - not ready
						success = 0
						return false, nil
					}
				}
			}
		}
		success++
		return success > 2, nil
	})

}
