package compat_otp

import (
	"context"
	"time"

	exutil "github.com/openshift/origin/test/extended/util"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kutilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// RemoveStatefulSets deletes the given stateful sets in a namespace
func RemoveStatefulSets(oc *exutil.CLI, sets ...string) error {
	errs := []error{}
	for _, set := range sets {
		e2e.Logf("Removing stateful set %s/%s", oc.Namespace(), set)
		if err := oc.AdminKubeClient().AppsV1().StatefulSets(oc.Namespace()).Delete(context.Background(), set, metav1.DeleteOptions{}); err != nil {
			e2e.Logf("Error occurred removing stateful set: %v", err)
			errs = append(errs, err)
		}

		err := wait.PollImmediate(5*time.Second, 5*time.Minute, func() (bool, error) {
			pods, err := GetStatefulSetPods(oc, set)
			if err != nil {
				e2e.Logf("Unable to get pods for statefulset/%s: %v", set, err)
				return false, err
			}
			if len(pods.Items) > 0 {
				e2e.Logf("Waiting for pods for statefulset/%s to terminate", set)
				return false, nil
			}
			e2e.Logf("Pods for statefulset/%s have terminated", set)
			return true, nil
		})

		if err != nil {
			e2e.Logf("Error occurred waiting for pods to terminate for statefulset/%s: %v", set, err)
			errs = append(errs, err)
		}
	}

	if len(errs) != 0 {
		return kutilerrors.NewAggregate(errs)
	}

	return nil
}
