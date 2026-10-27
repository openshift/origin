package authentication

import (
	"context"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	exutil "github.com/openshift/origin/test/extended/util"
	operator "github.com/openshift/origin/test/extended/util/operator"
)

func waitForOperatorToPickUpChanges(ctx context.Context, oc *exutil.CLI, name string) error {
	progressCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	if err := exutil.WaitForOperatorProgressingTrue(progressCtx, oc.AdminConfigClient(), name); err != nil {
		g.GinkgoWriter.Printf("operator %s did not become Progressing=True (may have reconciled quickly): %v\n", name, err)
	}
	return operator.WaitForOperatorsToSettle(ctx, oc.AdminConfigClient(), 10)
}

// waitForDeploymentRollout waits for a new Deployment generation and its complete
// rollout. Capture previousGeneration before changing configuration that triggers
// a rollout so that ready replicas from the previous configuration cannot satisfy it.
func waitForDeploymentRollout(
	ctx context.Context,
	oc *exutil.CLI,
	name, namespace string,
	previousGeneration int64) error {
	g.GinkgoHelper()

	lastStatus := "deployment has not been read"
	err := wait.PollUntilContextTimeout(ctx, 10*time.Second, 10*time.Minute, true, func(ctx context.Context) (bool, error) {
		deployment, err := oc.AdminKubeClient().AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			lastStatus = fmt.Sprintf("getting deployment: %v", err)
			g.GinkgoWriter.Println(lastStatus)
			return false, nil
		}

		desired := int32(1)
		if deployment.Spec.Replicas != nil {
			desired = *deployment.Spec.Replicas
		}
		status := deployment.Status
		lastStatus = fmt.Sprintf("generation=%d (must exceed %d), observedGeneration=%d, desired=%d, replicas=%d, updated=%d, ready=%d, available=%d, unavailable=%d",
			deployment.Generation, previousGeneration, status.ObservedGeneration, desired,
			status.Replicas, status.UpdatedReplicas, status.ReadyReplicas, status.AvailableReplicas, status.UnavailableReplicas)
		if deployment.Generation > previousGeneration &&
			status.ObservedGeneration >= deployment.Generation &&
			desired > 0 &&
			status.Replicas == desired &&
			status.UpdatedReplicas == desired &&
			status.ReadyReplicas == desired &&
			status.AvailableReplicas == desired &&
			status.UnavailableReplicas == 0 {
			return true, nil
		}
		g.GinkgoWriter.Printf("Waiting for deployment %s/%s rollout: %s\n", namespace, name, lastStatus)
		return false, nil
	})
	if err != nil {
		return fmt.Errorf("waiting for deployment %s/%s rollout: %s: %w", namespace, name, lastStatus, err)
	}
	return nil
}
