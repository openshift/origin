package authentication

import (
	"context"
	"fmt"

	g "github.com/onsi/ginkgo/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	exutil "github.com/openshift/origin/test/extended/util"
)

// Wrapper decorates a mutation (next) with behavior that must run around it, such as
// capturing pre-mutation state or waiting for the cluster to reconcile afterwards.
type Wrapper func(ctx context.Context, next func() error) error

// Chain composes wrappers into a single Wrapper. The FIRST wrapper is the OUTERMOST:
// it runs its pre-next code first and its post-next code last. Given
// Chain(A, B)(ctx, f) the execution order is:
//
//	A-before -> B-before -> f -> B-after -> A-after
//
// so order the wrappers from outermost concern to innermost.
func Chain(wrappers ...Wrapper) Wrapper {
	return func(ctx context.Context, final func() error) error {
		g.GinkgoHelper()

		current := final
		for i := len(wrappers) - 1; i >= 0; i-- {
			w := wrappers[i]
			next := current
			current = func() error {
				return w(ctx, next)
			}
		}
		return current()
	}
}

// OperatorChangesWaiter is a Wrapper source that applies a mutation and then waits for
// the named cluster operator to pick the change up and settle.
type OperatorChangesWaiter struct {
	oc           *exutil.CLI
	operatorName string
}

// WaitChanges runs mutate, then waits for the operator to reconcile and settle. As a
// Wrapper it should be placed INNER of a rollout wrapper so the operator's
// Progressing=True window is observed right after the mutation (see Chain ordering).
func (ow OperatorChangesWaiter) WaitChanges(
	ctx context.Context,
	mutate func() error,
) error {
	g.GinkgoHelper()
	g.By("Applying changes to operator " + ow.operatorName)
	err := mutate()
	if err != nil {
		return fmt.Errorf("mutation error: %w", err)
	}

	g.By("Waiting for operator " + ow.operatorName + " to pick up changes and stabilize")
	return waitForOperatorToPickUpChanges(ctx, ow.oc, ow.operatorName)
}

// RolloutWaiter is a Wrapper source that captures a Deployment's generation before a
// mutation and then waits for that Deployment to complete a new rollout.
type RolloutWaiter struct {
	oc                                  *exutil.CLI
	deploymentNamespace, deploymentName string
}

// MutateAndWaitForDeploymentRollout captures the Deployment generation, runs mutate
// (which may itself wait for the operator when wrapped via Chain), then waits for the
// Deployment to roll past the captured generation. Capturing before mutate keeps ready
// replicas from the previous configuration from satisfying the rollout check. As a
// Wrapper it should be the OUTERMOST so the generation is captured before any inner
// mutation runs.
func (rw RolloutWaiter) MutateAndWaitForDeploymentRollout(
	ctx context.Context,
	mutate func() error,
) error {
	g.GinkgoHelper()

	deployment, err := rw.oc.AdminKubeClient().
		AppsV1().
		Deployments(rw.deploymentNamespace).
		Get(ctx, rw.deploymentName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("getting deployment %s/%s before mutation: %w", rw.deploymentNamespace, rw.deploymentName, err)
	}
	previousGeneration := deployment.Generation

	if err := mutate(); err != nil {
		return fmt.Errorf("applying mutation: %w", err)
	}

	return waitForDeploymentRollout(ctx, rw.oc, rw.deploymentName, rw.deploymentNamespace, previousGeneration)
}
