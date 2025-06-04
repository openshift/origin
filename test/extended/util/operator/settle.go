package operator

import (
	"context"
	"fmt"
	"strings"
	"time"

	configv1 "github.com/openshift/api/config/v1"
	clientconfigv1 "github.com/openshift/client-go/config/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/kubernetes/test/e2e/framework"
)

func WaitForOperatorsToSettleWithDefaultClient(ctx context.Context) error {
	cfg := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(clientcmd.NewDefaultClientConfigLoadingRules(), &clientcmd.ConfigOverrides{})
	clusterConfig, err := cfg.ClientConfig()
	if err != nil {
		return fmt.Errorf("could not load client configuration: %v", err)
	}
	configClient, err := clientconfigv1.NewForConfig(clusterConfig)
	if err != nil {
		return err
	}
	return WaitForOperatorsToSettle(ctx, configClient)
}

// can be overridden for tests
var nowFn = realNow

func realNow() time.Time {
	return time.Now()
}

func WaitForOperatorsToSettle(ctx context.Context, configClient clientconfigv1.Interface) error {
	framework.Logf("Waiting for operators to settle")
	unsettledOperatorStatus := []string{}
	if err := wait.PollImmediate(10*time.Second, 5*time.Minute, func() (bool, error) {
		coList, err := configClient.ConfigV1().ClusterOperators().List(ctx, metav1.ListOptions{})
		if err != nil {
			framework.Logf("error getting ClusterOperators %v", err)
			return false, nil
		}
		unsettledOperatorStatus = unsettledOperators(coList.Items)
		done := len(unsettledOperatorStatus) == 0
		return done, nil

	}); err != nil {
		return fmt.Errorf("ClusterOperators did not settle: \n%v", strings.Join(unsettledOperatorStatus, "\n\t"))
	}
	return nil
}

func unsettledOperators(operators []configv1.ClusterOperator) []string {
	unsettledOperatorStatus := []string{}
	for _, co := range operators {
		available := findCondition(co.Status.Conditions, configv1.OperatorAvailable)
		degraded := findCondition(co.Status.Conditions, configv1.OperatorDegraded)
		progressing := findCondition(co.Status.Conditions, configv1.OperatorProgressing)
		if available.Status == configv1.ConditionTrue &&
			degraded.Status == configv1.ConditionFalse &&
			progressing.Status == configv1.ConditionFalse {
			continue
		}
		if available.Status != configv1.ConditionTrue {
			unsettledOperatorStatus = append(unsettledOperatorStatus, fmt.Sprintf("clusteroperator/%v is not Available for %v because %q", co.Name, nowFn().Sub(available.LastTransitionTime.Time), available.Message))
		}
		if degraded.Status != configv1.ConditionFalse {
			unsettledOperatorStatus = append(unsettledOperatorStatus, fmt.Sprintf("clusteroperator/%v is Degraded for %v because %q", co.Name, nowFn().Sub(degraded.LastTransitionTime.Time), degraded.Message))
		}
		if progressing.Status != configv1.ConditionFalse {
			unsettledOperatorStatus = append(unsettledOperatorStatus, fmt.Sprintf("clusteroperator/%v is Progressing for %v because %q", co.Name, nowFn().Sub(progressing.LastTransitionTime.Time), progressing.Message))
		}
	}
	return unsettledOperatorStatus
}

func findCondition(conditions []configv1.ClusterOperatorStatusCondition, name configv1.ClusterStatusConditionType) *configv1.ClusterOperatorStatusCondition {
	for i := range conditions {
		if name == conditions[i].Type {
			return &conditions[i]
		}
	}
	return nil
}

func conditionHasStatus(c *configv1.ClusterOperatorStatusCondition, status configv1.ConditionStatus) bool {
	if c == nil {
		return false
	}
	return c.Status == status
}
