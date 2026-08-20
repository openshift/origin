package apis

import (
	"context"
	"fmt"
	"strings"
	"time"

	operatorv1 "github.com/openshift/api/operator/v1"
	exutil "github.com/openshift/origin/test/extended/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
)

const (
	PacemakerHealthCheckDegradedCondition = "PacemakerHealthCheckDegraded"

	healthCheckPollInterval = 10 * time.Second
)

func getEtcdOperator(oc *exutil.CLI) (*operatorv1.Etcd, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return oc.AdminOperatorClient().OperatorV1().Etcds().Get(ctx, "cluster", metav1.GetOptions{})
}

func findOperatorCondition(etcd *operatorv1.Etcd, condType string) *operatorv1.OperatorCondition {
	for i := range etcd.Status.Conditions {
		if etcd.Status.Conditions[i].Type == condType {
			return &etcd.Status.Conditions[i]
		}
	}
	return nil
}

// describeNotDegradedCondition formats a not-yet-True PacemakerHealthCheckDegraded
// condition for logging. The operator's clearPacemakerDegradedCondition intentionally
// leaves Reason/Message empty when setting Status=False (see healthcheck.go), so an
// empty reason/message here reflects the healthy baseline, not a missing field bug.
func describeNotDegradedCondition(cond *operatorv1.OperatorCondition) string {
	if cond.Reason == "" && cond.Message == "" {
		return fmt.Sprintf("Status=%s (healthy)", cond.Status)
	}
	return fmt.Sprintf("Status=%s reason=%s message=%q", cond.Status, cond.Reason, cond.Message)
}

// WaitForPacemakerHealthCheckDegraded polls the etcd operator resource until
// PacemakerHealthCheckDegraded=True with a message containing expectedSubstring.
// Pass an empty expectedSubstring to accept any message.
func WaitForPacemakerHealthCheckDegraded(oc *exutil.CLI, expectedSubstring string, timeout time.Duration) error {
	deadline := time.After(timeout)
	ticker := time.NewTicker(healthCheckPollInterval)
	defer ticker.Stop()

	var lastErr string
	for {
		select {
		case <-deadline:
			return fmt.Errorf("timed out after %v waiting for PacemakerHealthCheckDegraded=True (last: %s)", timeout, lastErr)
		case <-ticker.C:
			etcd, err := getEtcdOperator(oc)
			if err != nil {
				lastErr = fmt.Sprintf("get etcd operator: %v", err)
				framework.Logf("WaitForPacemakerHealthCheckDegraded: %s", lastErr)
				continue
			}

			cond := findOperatorCondition(etcd, PacemakerHealthCheckDegradedCondition)
			if cond == nil {
				lastErr = "condition not found"
				framework.Logf("WaitForPacemakerHealthCheckDegraded: condition not yet present on etcd operator")
				continue
			}

			if cond.Status != operatorv1.ConditionTrue {
				lastErr = describeNotDegradedCondition(cond)
				framework.Logf("WaitForPacemakerHealthCheckDegraded: %s", lastErr)
				continue
			}

			if expectedSubstring != "" && !strings.Contains(cond.Message, expectedSubstring) {
				lastErr = fmt.Sprintf("True but message %q does not contain %q", cond.Message, expectedSubstring)
				framework.Logf("WaitForPacemakerHealthCheckDegraded: %s", lastErr)
				continue
			}

			framework.Logf("PacemakerHealthCheckDegraded=True confirmed (reason=%s, message=%q)", cond.Reason, cond.Message)
			return nil
		}
	}
}

// WaitForPacemakerHealthCheckCleared polls the etcd operator resource until
// PacemakerHealthCheckDegraded=False or the condition is absent.
func WaitForPacemakerHealthCheckCleared(oc *exutil.CLI, timeout time.Duration) error {
	deadline := time.After(timeout)
	ticker := time.NewTicker(healthCheckPollInterval)
	defer ticker.Stop()

	var lastErr string
	for {
		select {
		case <-deadline:
			return fmt.Errorf("timed out after %v waiting for PacemakerHealthCheckDegraded to clear (last: %s)", timeout, lastErr)
		case <-ticker.C:
			etcd, err := getEtcdOperator(oc)
			if err != nil {
				lastErr = fmt.Sprintf("get etcd operator: %v", err)
				framework.Logf("WaitForPacemakerHealthCheckCleared: %s", lastErr)
				continue
			}

			cond := findOperatorCondition(etcd, PacemakerHealthCheckDegradedCondition)
			if cond == nil {
				framework.Logf("PacemakerHealthCheckDegraded condition absent — treating as cleared")
				return nil
			}

			if cond.Status == operatorv1.ConditionFalse {
				framework.Logf("PacemakerHealthCheckDegraded=False confirmed")
				return nil
			}

			lastErr = fmt.Sprintf("Status=%s reason=%s message=%q", cond.Status, cond.Reason, cond.Message)
			framework.Logf("WaitForPacemakerHealthCheckCleared: still degraded — %s", lastErr)
		}
	}
}

// pacemakerHealthCheckEventNamespace is where the healthcheck controller's
// library-go event recorder writes events — the operator's own pod namespace
// (openshift-etcd-operator), not the target namespace (openshift-etcd) that
// the status collector uses for its own PacemakerFailedResourceAction /
// PacemakerStatusCollectionError events.
const pacemakerHealthCheckEventNamespace = "openshift-etcd-operator"

// WaitForPacemakerEvent polls events in the openshift-etcd-operator namespace
// until one with the given Reason appears. This applies to healthcheck-controller
// reasons (e.g. PacemakerHealthy, PacemakerClusterInMaintenance, PacemakerNodeOffline).
func WaitForPacemakerEvent(oc *exutil.CLI, reason string, timeout time.Duration) error {
	deadline := time.After(timeout)
	ticker := time.NewTicker(healthCheckPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-deadline:
			return fmt.Errorf("timed out after %v waiting for event with reason %q in %s", timeout, reason, pacemakerHealthCheckEventNamespace)
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			events, err := oc.AdminKubeClient().CoreV1().Events(pacemakerHealthCheckEventNamespace).List(ctx, metav1.ListOptions{
				FieldSelector: fmt.Sprintf("reason=%s", reason),
			})
			cancel()
			if err != nil {
				framework.Logf("WaitForPacemakerEvent: list events: %v", err)
				continue
			}
			if len(events.Items) > 0 {
				latest := events.Items[len(events.Items)-1]
				framework.Logf("Found event reason=%s message=%q", latest.Reason, latest.Message)
				return nil
			}
		}
	}
}

// ExpectPacemakerHealthCheckNotDegraded checks the etcd operator resource
// and returns an error if PacemakerHealthCheckDegraded is True.
func ExpectPacemakerHealthCheckNotDegraded(oc *exutil.CLI) error {
	etcd, err := getEtcdOperator(oc)
	if err != nil {
		return fmt.Errorf("get etcd operator: %w", err)
	}

	cond := findOperatorCondition(etcd, PacemakerHealthCheckDegradedCondition)
	if cond == nil {
		return nil
	}

	if cond.Status == operatorv1.ConditionTrue {
		return fmt.Errorf("PacemakerHealthCheckDegraded=True (reason=%s, message=%q)", cond.Reason, cond.Message)
	}
	return nil
}
