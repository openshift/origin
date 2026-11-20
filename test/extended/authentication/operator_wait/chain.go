package operator_wait

import (
	"context"
	"errors"
	"fmt"
	"slices"

	g "github.com/onsi/ginkgo/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	exutil "github.com/openshift/origin/test/extended/util"
)

// Wrapper decorates a mutation (next) with behavior that must run around it, such as
// capturing pre-mutation state or waiting for the cluster to reconcile afterwards.
type Wrapper func(ctx context.Context, next func() error) error

// Chain composes wrappers into a single Wrapper. The FIRST wrapper is the OUTERMOST:
// given Chain(A, B)(ctx, f) the execution order is A-before -> B-before -> f -> B-after
// -> A-after. Order the wrappers from outermost concern to innermost; an error in a
// before step short-circuits everything inner to it and skips all afters.
func Chain(wrappers ...Wrapper) Wrapper {
	return func(ctx context.Context, final func() error) error {
		g.GinkgoHelper()

		current := final
		for _, w := range slices.Backward(wrappers) {
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
	cli          *exutil.CLI
	operatorName string
}

// NewOperatorChangesWaiter builds the waiter for the named cluster operator. It fails
// only when cli is nil.
func NewOperatorChangesWaiter(
	operatorName string,
	cli *exutil.CLI,
) (OperatorChangesWaiter, error) {
	toReturn := OperatorChangesWaiter{}
	if cli == nil {
		return toReturn, errors.New("no cli") //TODO use the Err defined into pr 31837
	}

	toReturn.cli = cli
	toReturn.operatorName = operatorName
	return toReturn, nil
}

// WaitChanges runs mutate, then waits for the operator to reconcile and settle. As a
// Wrapper it must be placed INNER of a rollout wrapper so the operator's
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
	return WaitForOperatorToPickUpChanges(ctx, ow.cli, ow.operatorName)
}

// RolloutWaiter is a Wrapper source that captures a Deployment's generation before a
// mutation and then waits for that Deployment to complete a new rollout.
type RolloutWaiter struct {
	cli                                 *exutil.CLI
	deploymentNamespace, deploymentName string
}

// NewRolloutWaiter builds the waiter for the named Deployment. It fails only when cli
// is nil.
func NewRolloutWaiter(
	deploymentNamespace, deploymentName string,
	cli *exutil.CLI,
) (RolloutWaiter, error) {
	toReturn := RolloutWaiter{}
	if cli == nil {
		return toReturn, errors.New("no cli") //TODO use the Err defined into pr 31837
	}

	toReturn.cli = cli
	toReturn.deploymentNamespace = deploymentNamespace
	toReturn.deploymentName = deploymentName
	return toReturn, nil
}

// MutateAndWaitForDeploymentRollout captures the Deployment generation before mutate
// runs, then waits for the Deployment to roll past the captured generation. Capturing
// before the mutation keeps ready replicas from the previous configuration from
// satisfying the rollout check. As a Wrapper it must be the OUTERMOST.
func (rw RolloutWaiter) MutateAndWaitForDeploymentRollout(
	ctx context.Context,
	mutate func() error,
) error {
	g.GinkgoHelper()

	deployment, err := rw.cli.AdminKubeClient().
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

	return WaitForDeploymentRollout(ctx, rw.cli, rw.deploymentName, rw.deploymentNamespace, previousGeneration)
}
