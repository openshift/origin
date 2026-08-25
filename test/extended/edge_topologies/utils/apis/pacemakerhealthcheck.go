package apis

import (
	"context"
	"fmt"
	"strings"
	"time"

	operatorv1 "github.com/openshift/api/operator/v1"
	exutil "github.com/openshift/origin/test/extended/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
)

const (
	PacemakerHealthCheckDegradedCondition = "PacemakerHealthCheckDegraded"

	healthCheckPollInterval = 10 * time.Second

	// pacemakerTargetNamespace is where the status-collector CronJob and the
	// PacemakerCluster data pipeline run.
	pacemakerTargetNamespace = "openshift-etcd"

	// statusCollectorCronJobName is the CronJob that snapshots pacemaker status
	// into the PacemakerCluster CR (see cluster-etcd-operator).
	statusCollectorCronJobName = "pacemaker-status-collector"
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

// dumpHealthCheckDiagnostics logs the state most useful for triaging why a
// PacemakerHealthCheckDegraded transition did not happen within the timeout: the
// etcd operator condition, the PacemakerCluster CR staleness and conditions, the
// status-collector CronJob's last run, and node readiness. Together these show
// whether the data pipeline was flowing (CR fresh, CronJob running, nodes Ready)
// or broken. Every step is best-effort — this runs on an already-failing path and
// must never itself fail or panic.
func dumpHealthCheckDiagnostics(oc *exutil.CLI, reason string) {
	framework.Logf("========== PACEMAKER HEALTHCHECK DIAGNOSTICS (%s) ==========", reason)

	// 1. etcd operator PacemakerHealthCheckDegraded condition
	if etcd, err := getEtcdOperator(oc); err != nil {
		framework.Logf("diagnostics: get etcd operator: %v", err)
	} else if cond := findOperatorCondition(etcd, PacemakerHealthCheckDegradedCondition); cond == nil {
		framework.Logf("diagnostics: etcd operator %s condition absent", PacemakerHealthCheckDegradedCondition)
	} else {
		framework.Logf("diagnostics: etcd operator %s: Status=%s reason=%s message=%q lastTransition=%s",
			PacemakerHealthCheckDegradedCondition, cond.Status, cond.Reason, cond.Message,
			cond.LastTransitionTime.Format(time.RFC3339))
	}

	// 2. PacemakerCluster CR staleness and conditions
	if pc, err := GetPacemakerCluster(oc); err != nil {
		framework.Logf("diagnostics: get PacemakerCluster CR: %v", err)
	} else {
		lastUpdated := pc.Status.LastUpdated.Time
		if lastUpdated.IsZero() {
			framework.Logf("diagnostics: PacemakerCluster CR lastUpdated is zero (status never populated)")
		} else {
			framework.Logf("diagnostics: PacemakerCluster CR lastUpdated=%s (age %v)",
				lastUpdated.Format(time.RFC3339), time.Since(lastUpdated).Round(time.Second))
		}
		for i := range pc.Status.Conditions {
			c := &pc.Status.Conditions[i]
			framework.Logf("diagnostics: PacemakerCluster condition %s=%s reason=%s message=%q",
				c.Type, c.Status, c.Reason, c.Message)
		}
	}

	// 3. status-collector CronJob last run
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	cronJob, err := oc.AdminKubeClient().BatchV1().CronJobs(pacemakerTargetNamespace).Get(ctx, statusCollectorCronJobName, metav1.GetOptions{})
	cancel()
	if err != nil {
		framework.Logf("diagnostics: get CronJob %s/%s: %v", pacemakerTargetNamespace, statusCollectorCronJobName, err)
	} else {
		lastSchedule := "never"
		if cronJob.Status.LastScheduleTime != nil {
			lastSchedule = cronJob.Status.LastScheduleTime.Format(time.RFC3339)
		}
		lastSuccess := "never"
		if cronJob.Status.LastSuccessfulTime != nil {
			lastSuccess = cronJob.Status.LastSuccessfulTime.Format(time.RFC3339)
		}
		framework.Logf("diagnostics: CronJob %s lastSchedule=%s lastSuccessful=%s activeJobs=%d",
			statusCollectorCronJobName, lastSchedule, lastSuccess, len(cronJob.Status.Active))
	}

	// 4. Node readiness
	ctx2, cancel2 := context.WithTimeout(context.Background(), 30*time.Second)
	nodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(ctx2, metav1.ListOptions{})
	cancel2()
	if err != nil {
		framework.Logf("diagnostics: list nodes: %v", err)
	} else {
		for i := range nodes.Items {
			n := &nodes.Items[i]
			ready := "Unknown"
			for _, c := range n.Status.Conditions {
				if c.Type == corev1.NodeReady {
					ready = string(c.Status)
					break
				}
			}
			framework.Logf("diagnostics: node %s Ready=%s", n.Name, ready)
		}
	}

	framework.Logf("========== END PACEMAKER HEALTHCHECK DIAGNOSTICS ==========")
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
			dumpHealthCheckDiagnostics(oc, "WaitForPacemakerHealthCheckDegraded timeout")
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
			dumpHealthCheckDiagnostics(oc, "WaitForPacemakerHealthCheckCleared timeout")
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

// eventTime returns the most recent activity timestamp for an event, preferring
// EventTime, then LastTimestamp, then the object's creation timestamp. This is
// used to distinguish freshly-emitted events from stale ones left over from an
// earlier reconcile or a previous test run.
func eventTime(ev *corev1.Event) time.Time {
	if !ev.EventTime.IsZero() {
		return ev.EventTime.Time
	}
	if !ev.LastTimestamp.IsZero() {
		return ev.LastTimestamp.Time
	}
	return ev.CreationTimestamp.Time
}

// WaitForPacemakerEvent polls events in the openshift-etcd-operator namespace
// until one with the given Reason emitted at or after the provided lower bound
// (since) appears. This applies to healthcheck-controller reasons (e.g.
// PacemakerHealthy, PacemakerClusterInMaintenance, PacemakerNodeOffline). The
// since bound prevents a stale event from a prior reconcile or test from
// satisfying the wait.
func WaitForPacemakerEvent(oc *exutil.CLI, reason string, since time.Time, timeout time.Duration) error {
	deadline := time.After(timeout)
	ticker := time.NewTicker(healthCheckPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-deadline:
			return fmt.Errorf("timed out after %v waiting for event with reason %q emitted at or after %s in %s",
				timeout, reason, since.Format(time.RFC3339), pacemakerHealthCheckEventNamespace)
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
			for i := range events.Items {
				ev := &events.Items[i]
				if eventTime(ev).Before(since) {
					continue
				}
				framework.Logf("Found event reason=%s message=%q at %s (baseline %s)",
					ev.Reason, ev.Message, eventTime(ev).Format(time.RFC3339), since.Format(time.RFC3339))
				return nil
			}
		}
	}
}

// IsPacemakerHealthCheckDegraded performs a single check of the etcd operator
// resource and reports whether PacemakerHealthCheckDegraded is currently True,
// along with the condition message. A missing condition is reported as not
// degraded. Unlike WaitForPacemakerHealthCheckDegraded, this does not poll, so
// callers can interleave it with other actions (e.g. re-inducing a failure that
// Pacemaker would otherwise auto-recover before the next status snapshot).
func IsPacemakerHealthCheckDegraded(oc *exutil.CLI) (bool, string, error) {
	etcd, err := getEtcdOperator(oc)
	if err != nil {
		return false, "", fmt.Errorf("get etcd operator: %w", err)
	}

	cond := findOperatorCondition(etcd, PacemakerHealthCheckDegradedCondition)
	if cond == nil {
		return false, "", nil
	}

	return cond.Status == operatorv1.ConditionTrue, cond.Message, nil
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
