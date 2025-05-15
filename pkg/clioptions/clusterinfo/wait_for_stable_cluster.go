package clusterinfo

import (
	"context"
	"fmt"
	"strings"
	"time"

	configv1 "github.com/openshift/api/config/v1"
	configclient "github.com/openshift/client-go/config/clientset/versioned"
	"github.com/openshift/library-go/pkg/config/clusteroperator/v1helpers"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	exutil "github.com/openshift/origin/test/extended/util"
)

func unstableOperatorReason(operator *configv1.ClusterOperator) string {
	notableConditions := []string{}
	if !v1helpers.IsStatusConditionTrue(operator.Status.Conditions, configv1.OperatorAvailable) {
		notableConditions = append(notableConditions, "unavailable")
	}
	if !v1helpers.IsStatusConditionFalse(operator.Status.Conditions, configv1.OperatorProgressing) {
		notableConditions = append(notableConditions, "progressing")
	}
	if !v1helpers.IsStatusConditionFalse(operator.Status.Conditions, configv1.OperatorDegraded) {
		notableConditions = append(notableConditions, "degraded")
	}

	return strings.Join(notableConditions, " and ")
}

func summerizeUnstableOperators(operators map[string]string) string {
	msg := ""
	for operator, reason := range operators {
		msg += fmt.Sprintf("\noperator %s is not stable: %s", operator, reason)
	}
	return msg
}

// WaitForStableCluster checks to make sure all operators are stable. If not, it will wait.
// It will generate success junit if all operators are stable on the first check.
// It will generate flake junits if some operators recovered from unstable conditions while it waits.
// It will generate failure junit if any operators are still unstable after timeout.
func WaitForStableCluster(ctx context.Context, config *rest.Config) ([]*junitapi.JUnitTestCase, error) {
	const testName = "Cluster should be stable after installation is complete"
	// Create two different junit test name for easy analysis
	const testNameUnrecovered = "Cluster should be stable before test is started"

	// Skip microshift
	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return []*junitapi.JUnitTestCase{
			{
				Name: testName,
				FailureOutput: &junitapi.FailureOutput{
					Output: fmt.Sprintf("error creating kube client: %v", err),
				},
			},
			{
				Name: testNameUnrecovered,
				FailureOutput: &junitapi.FailureOutput{
					Output: fmt.Sprintf("error creating kube client: %v", err),
				},
			},
		}, err
	}
	isMicroShift, err := exutil.IsMicroShiftCluster(kubeClient)
	if err != nil {
		return []*junitapi.JUnitTestCase{
			{
				Name: testName,
				FailureOutput: &junitapi.FailureOutput{
					Output: fmt.Sprintf("error determining microshift cluster: %v", err),
				},
			},
			{
				Name: testNameUnrecovered,
				FailureOutput: &junitapi.FailureOutput{
					Output: fmt.Sprintf("error determining microshift cluster: %v", err),
				},
			},
		}, err
	}
	if isMicroShift {
		return []*junitapi.JUnitTestCase{
			{
				Name: testName,
			},
			{
				Name: testNameUnrecovered,
			},
		}, nil
	}

	configClient, err := configclient.NewForConfig(config)
	if err != nil {
		return []*junitapi.JUnitTestCase{
			{
				Name: testName,
				FailureOutput: &junitapi.FailureOutput{
					Output: fmt.Sprintf("error creating config client: %v", err),
				},
			},
			{
				Name: testNameUnrecovered,
				FailureOutput: &junitapi.FailureOutput{
					Output: fmt.Sprintf("error creating config client: %v", err),
				},
			},
		}, err
	}

	interval := 10 * time.Second
	timeout := 10 * time.Minute
	minimumStablePeriod := 3 * time.Minute
	recoveredOperators := map[string]string{}
	unstableOperators := map[string]string{}
	var stabilityStarted *time.Time
	pollingStarted := time.Now()
	waitErr := wait.PollUntilContextTimeout(ctx, interval, timeout, true, func(waitCtx context.Context) (bool, error) {
		operators, err := configClient.ConfigV1().ClusterOperators().List(waitCtx, metav1.ListOptions{})
		if err != nil {
			if waitCtx.Err() == context.DeadlineExceeded {
				return false, fmt.Errorf("some operators were unstable for too long")
			}
			stabilityStarted = nil
			return false, fmt.Errorf("failed to list clusteroperators: %v", err)
		}
		now := time.Now()
		for _, operator := range operators.Items {
			if unstableReason := unstableOperatorReason(&operator); len(unstableReason) > 0 {
				if _, ok := unstableOperators[operator.Name]; !ok {
					unstableOperators[operator.Name] = unstableReason
				}
			} else {
				if unstableReason, ok := unstableOperators[operator.Name]; ok {
					recoveredOperators[operator.Name] = unstableReason
					delete(unstableOperators, operator.Name)
				}
			}
		}
		if len(unstableOperators) > 0 {
			stabilityStarted = nil
			return false, nil
		}
		if stabilityStarted == nil {
			stabilityStarted = &now
		}
		timeStable := now.Sub(*stabilityStarted)
		if timeStable < minimumStablePeriod {
			return false, nil
		}
		return true, nil
	})
	timingMsg := ""
	if stabilityStarted != nil {
		timingMsg = fmt.Sprintf(", unstable duration %s", stabilityStarted.Sub(pollingStarted))
	}
	if waitErr != nil {
		msg := fmt.Sprintf("error waiting for cluster operators to become stable: %v", waitErr)
		if len(unstableOperators) > 0 {
			msg += fmt.Sprintf("\nunstable operators that never recovered:\n")
			msg += summerizeUnstableOperators(unstableOperators)
		}
		if len(recoveredOperators) > 0 {
			msg += fmt.Sprintf("\nunstable operators that recovered%s:\n", timingMsg)
			msg += summerizeUnstableOperators(recoveredOperators)
		}
		// Flake for now.
		return []*junitapi.JUnitTestCase{
			{
				Name: testName,
			},
			{
				Name: testName,
				FailureOutput: &junitapi.FailureOutput{
					Output: msg,
				},
			},
			{
				Name: testNameUnrecovered,
			},
			{
				Name: testNameUnrecovered,
				FailureOutput: &junitapi.FailureOutput{
					Output: msg,
				},
			},
		}, waitErr
	}
	if len(recoveredOperators) == 0 {
		return []*junitapi.JUnitTestCase{
			{
				Name: testName,
			},
			{
				Name: testNameUnrecovered,
			},
		}, nil
	}
	// Some operators recovered from unstable conditions before timeout
	msg := fmt.Sprintf("some operators were unstable but recovered%s\n%s", timingMsg, summerizeUnstableOperators(recoveredOperators))
	return []*junitapi.JUnitTestCase{
		{
			Name: testName,
		},
		{
			Name: testName,
			FailureOutput: &junitapi.FailureOutput{
				Output: msg,
			},
		},
		{
			Name: testNameUnrecovered,
		},
	}, nil
}
