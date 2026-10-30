package nvidia

import (
	"context"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo/v2"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
)

// registerPodCleanup waits for the claim to be released before the next serial spec starts.
func registerPodCleanup(client kubernetes.Interface, namespace, podName, claimName string) {
	registerPodsCleanup(client, namespace, claimName, podName)
}

// registerPodsCleanup deletes all pods using one claim before waiting for the claim to be released.
// This keeps the cleanup correct for tests that deliberately share a claim between pods.
func registerPodsCleanup(client kubernetes.Interface, namespace, claimName string, podNames ...string) {
	g.DeferCleanup(func(ctx context.Context) error {
		for _, podName := range podNames {
			err := client.CoreV1().Pods(namespace).Delete(ctx, podName, metav1.DeleteOptions{})
			if err != nil && !apierrors.IsNotFound(err) {
				return fmt.Errorf("delete pod %s/%s: %w", namespace, podName, err)
			}
			if err == nil {
				if err := e2epod.WaitForPodNotFoundInNamespace(ctx, client, podName, namespace, 2*time.Minute); err != nil {
					return fmt.Errorf("wait for pod %s/%s deletion: %w", namespace, podName, err)
				}
			}
		}
		err := wait.PollUntilContextTimeout(ctx, 2*time.Second, 2*time.Minute, true, func(ctx context.Context) (bool, error) {
			claim, err := client.ResourceV1().ResourceClaims(namespace).Get(ctx, claimName, metav1.GetOptions{})
			if apierrors.IsNotFound(err) {
				return true, nil
			}
			if err != nil {
				return false, err
			}
			return claim.Status.Allocation == nil && len(claim.Status.ReservedFor) == 0, nil
		})
		if err != nil {
			return fmt.Errorf("wait for claim %s/%s release: %w", namespace, claimName, err)
		}
		return nil
	})
}
